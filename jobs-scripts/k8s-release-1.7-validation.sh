#!/bin/bash
set -o xtrace

VC_IP=0.0.0.0
DOCKER_USERNAME='*******'
DCOKER_PASSWORD='*******'
DATA_CENTER='vcqaDC'
DATASTORE='vsanDatastore'
CLUSTER_NAME='cluster-vsan-1'
DOCKER_REGISTRY='docker.io/divyen'

# cleanup
docker rm $(docker ps -a -q)
docker rmi $(docker images -q)
docker volume rm $(docker volume ls -qf dangling=true)

K8S_BUILD=`curl -s https://storage.googleapis.com/kubernetes-release-dev/ci/latest-1.7.txt`
echo $K8S_BUILD
IMAGE_TAG="nightly-1.7"

cd /root || exit
# Install kubectl Binary via curl
curl -LO https://storage.googleapis.com/kubernetes-release-dev/ci/$K8S_BUILD/bin/linux/amd64/kubectl
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl
kubectl version --client

# Building kubernetes Image
cd /opt || exit
git clone --depth 1 https://github.com/kubernetes/kubernetes.git
cd /opt/kubernetes || exit
mkdir -p _output/dockerized/bin/linux/amd64
cd _output/dockerized/bin/linux/amd64 || exit
curl -LO https://storage.googleapis.com/kubernetes-release-dev/ci/$K8S_BUILD/bin/linux/amd64/hyperkube
cd /opt/kubernetes/cluster/images/hyperkube || exit
make build VERSION=$IMAGE_TAG

# uploading kubernetes image to docker hub
docker login --username "$DOCKER_USERNAME" --password "$DCOKER_PASSWORD"
docker tag gcr.io/google_containers/hyperkube-amd64:$IMAGE_TAG divyen/hyperkube-amd64:$IMAGE_TAG
docker push divyen/hyperkube-amd64:$IMAGE_TAG

# clean up local image
docker rmi gcr.io/google_containers/hyperkube-amd64:$IMAGE_TAG
docker rmi divyen/hyperkube-amd64:$IMAGE_TAG

# Cleaning up VMs from previous build
export GOVC_INSECURE=1
export GOVC_URL='https://Administrator@vsphere.local:Admin!23@'$VC_IP'/sdk'

MASTER_VM=`govc vm.info kubernetes/kubernetes-master`
NODE1_VM=`govc vm.info kubernetes/kubernetes-node1`
NODE2_VM=`govc vm.info kubernetes/kubernetes-node2`
NODE3_VM=`govc vm.info kubernetes/kubernetes-node3`
NODE4_VM=`govc vm.info kubernetes/kubernetes-node4`

# power off and delete VMs from previous build
if [[ ! -z $MASTER_VM ]]; then
    govc vm.destroy kubernetes/kubernetes-master
fi

if [[ ! -z $NODE1_VM ]]; then
    govc vm.destroy kubernetes/kubernetes-node1
fi

if [[ ! -z $NODE2_VM ]]; then
    govc vm.destroy kubernetes/kubernetes-node2
fi

if [[ ! -z $NODE3_VM ]]; then
    govc vm.destroy kubernetes/kubernetes-node3
fi

if [[ ! -z $NODE4_VM ]]; then
    govc vm.destroy kubernetes/kubernetes-node4
fi

cd /mnt/gitsource || exit
# cleaning up traces from Previous Run
rm -rf /mnt/gitsource/kubernetes-anywhere
rm -rf /mnt/gitsource/kubernetes-anywhere-config
mkdir /mnt/gitsource/kubernetes-anywhere-config
cd /mnt/gitsource/kubernetes-anywhere-config || exit

# downloading kubectl
wget https://storage.googleapis.com/kubernetes-release-dev/ci/${K8S_BUILD}/bin/linux/amd64/kubectl -O kubectl_nightly
chmod +x /mnt/gitsource/kubernetes-anywhere-config/kubectl_nightly
cp /mnt/gitsource/kubernetes-anywhere-config/kubectl_nightly /usr/local/bin/kubectl
kubectl version --client

# Deploying kubernetes cluster
touch ./.config
echo '.phase1.num_nodes=4' >> .config
echo '.phase1.cluster_name="kubernetes"' >> .config
echo '.phase1.ssh_user=""' >> .config
echo '.phase1.cloud_provider="vsphere"' >> .config
echo '.phase1.vSphere.url="'$VC_IP'"' >> .config
echo '.phase1.vSphere.port="443"' >> .config
echo '.phase1.vSphere.username="Administrator@vsphere.local"' >> .config
echo '.phase1.vSphere.password="Admin!23"' >> .config
echo '.phase1.vSphere.insecure=y' >> .config
echo '.phase1.vSphere.datacenter="'$DATA_CENTER'"' >> .config
echo '.phase1.vSphere.datastore="'$DATASTORE'"' >> .config
echo '.phase1.vSphere.placement="cluster"' >> .config
echo '.phase1.vSphere.cluster="'$CLUSTER_NAME'"' >> .config
echo '.phase1.vSphere.useresourcepool="no"' >> .config
echo '.phase1.vSphere.vcpu="2"' >> .config
echo '.phase1.vSphere.memory="2048"' >> .config
echo '.phase1.vSphere.network="VM Network"' >> .config
echo '.phase1.vSphere.vmfolderpath="kubernetes"' >> .config
echo '.phase1.vSphere.template="KubernetesAnywhereTemplatePhotonOS"' >> .config
echo '.phase1.vSphere.flannel_net="172.1.0.0/16"' >> .config

echo '.phase2.docker_registry="'$DOCKER_REGISTRY'"' >> .config
echo '.phase2.kubernetes_version="'$IMAGE_TAG'"' >> .config
echo '.phase2.installer_container="docker.io/divyen/k8s-ignition:nightly"'>> .config
echo '.phase2.provider="ignition"' >> .config

echo '.phase3.run_addons=y' >> .config
echo '.phase3.kube_proxy=y' >> .config
echo '.phase3.dashboard=y' >> .config
echo '.phase3.heapster=y' >> .config
echo '.phase3.kube_dns=y' >> .config
echo '.phase3.weave_net=n' >> .config

docker pull cnastorage/kubernetes-anywhere:latest
docker run --volume="/root/jenkins-slave/gitsource/kubernetes-anywhere-config:/opt/kubernetes-anywhere-config" cnastorage/kubernetes-anywhere:latest "/bin/bash" "-c" "cat /opt/kubernetes-anywhere-config/.config > .config && cat .config && rm -rf /usr/local/bin/kubectl && cp /opt/kubernetes-anywhere-config/kubectl_nightly /usr/local/bin/kubectl && make deploy && cp /opt/kubernetes-anywhere/phase1/vsphere/kubernetes/kubeconfig.json /opt/kubernetes-anywhere-config/"
export KUBECONFIG=/mnt/gitsource/kubernetes-anywhere-config/kubeconfig.json
cat /mnt/gitsource/kubernetes-anywhere-config/kubeconfig.json
kubectl cluster-info
kubectl get nodes

# Buding E2E binary
cd /opt/kubernetes || exit
make quick-release

# Executing E2E tests
export KUBERNETES_CONFORMANCE_PROVIDER="vsphere"
export KUBERNETES_CONFORMANCE_TEST=Y
export VSPHERE_VCENTER=$VC_IP
export VSPHERE_VCENTER_PORT=443
export VSPHERE_USER=Administrator@vsphere.local
export VSPHERE_PASSWORD='Admin!23'
export VSPHERE_DATACENTER=vcqaDC
export VSPHERE_DATASTORE=vsanDatastore
export VSPHERE_INSECURE=true
export VSPHERE_WORKING_DIR='/vcqaDC/vm/kubernetes/'
export VSPHERE_SECOND_SHARED_DATASTORE=sharedVmfs-0
export VSPHERE_SPBM_TAG_POLICY=tagbased
export VSPHERE_SPBM_GOLD_POLICY=gold
export VSPHERE_VM_NAME="dummy"

export TEST_BEGIN="-------------------------------[ Starting the Test ]-------------------------------"
export TEST_END="-------------------------------[ End of the Test ]-------------------------------"

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_vsan_policy.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=vSphere\sStorage\spolicy\ssupport\sfor\sdynamic\sprovisioning'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_placement.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=Volume\sPlacement'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_diskformat.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=Volume\sDisk\sFormat'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_fstype.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=vsphere\sVolume\sfstype'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/pvc_label_selector.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=Selector-Label\sVolume\sBinding:vsphere'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/persistent_volumes-vsphere.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=PersistentVolumes:vsphere'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/pv_reclaimpolicy.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=persistentvolumereclaim'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/volumes.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=vsphere\sshould\sbe\smountable'
echo $TEST_END

echo "Running : https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_ops_storm.go"
echo $TEST_BEGIN
go run hack/e2e.go --check-version-skew=false -v -test --test_args='--ginkgo.focus=vsphere\svolume\soperations\sstorm'
echo $TEST_END

# printing kubeconfig.json and cluster status
cat /mnt/gitsource/kubernetes-anywhere-config/kubeconfig.json
kubectl cluster-info
kubectl get nodes
