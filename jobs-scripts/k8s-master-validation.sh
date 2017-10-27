#!/bin/bash
set -o xtrace
VC_IP=0.0.0.0
DATA_CENTER='vcqaDC'
DATASTORE='vsanDatastore'
CLUSTER_NAME='cluster-vsan-1'
DOCKER_REGISTRY='docker.io/divyen'
DOCKER_USERNAME='*******'
DCOKER_PASSWORD='*******'

# cleanup
docker rm $(docker ps -a -q)
docker rmi $(docker images -q)
docker volume rm $(docker volume ls -qf dangling=true)

K8S_BUILD=`curl -s https://storage.googleapis.com/kubernetes-release-dev/ci/latest-green.txt`
echo $K8S_BUILD
IMAGE_TAG="ci-master-nightly"

cd /root || exit
# Install kubectl Binary via curl
curl -LO https://storage.googleapis.com/kubernetes-release-dev/ci/$K8S_BUILD/bin/linux/amd64/kubectl

chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl
kubectl version --client

# Building kubernetes Image
cd /mnt/gitsource || exit
rm -rf /mnt/gitsource/kubernetes
git clone https://github.com/kubernetes/kubernetes.git
cd /mnt/gitsource/kubernetes || exit
make quick-release || exit

cd /mnt/gitsource/kubernetes/cluster/images/hyperkube || exit
make build VERSION=$IMAGE_TAG || exit
sleep 15
docker images
# uploading kubernetes image to docker hub
docker login --username "$DOCKER_USERNAME" --password "$DCOKER_PASSWORD"
docker tag gcr.io/google-containers/hyperkube-amd64:$IMAGE_TAG divyen/hyperkube-amd64:$IMAGE_TAG
docker push divyen/hyperkube-amd64:$IMAGE_TAG
docker rmi gcr.io/google-containers/hyperkube-amd64:$IMAGE_TAG
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
echo '.phase2.installer_container="docker.io/cnastorage/k8s-ignition:v1.8-dev-release"'>> .config
echo '.phase2.provider="ignition"' >> .config
echo '.phase3.run_addons=n' >> .config
echo '.phase3.kube_proxy=n' >> .config
echo '.phase3.dashboard=n' >> .config
echo '.phase3.heapster=n' >> .config
echo '.phase3.kube_dns=n' >> .config
echo '.phase3.weave_net=n' >> .config

docker pull cnastorage/kubernetes-anywhere:latest
docker run --volume="/root/jenkins-slave/gitsource/kubernetes-anywhere-config:/opt/kubernetes-anywhere-config" cnastorage/kubernetes-anywhere:latest "/bin/bash" "-c" "cat /opt/kubernetes-anywhere-config/.config > .config && cat .config && rm -rf /usr/local/bin/kubectl && cp /opt/kubernetes-anywhere-config/kubectl_nightly /usr/local/bin/kubectl && make deploy && cp /opt/kubernetes-anywhere/phase1/vsphere/kubernetes/kubeconfig.json /opt/kubernetes-anywhere-config/"
export KUBECONFIG=/mnt/gitsource/kubernetes-anywhere-config/kubeconfig.json
cat /mnt/gitsource/kubernetes-anywhere-config/kubeconfig.json

echo "Wait for 5 minutes to allow Cluster to become Ready"
sleep 5m

kubectl cluster-info || exit
kubectl get nodes || exit

# configuring password less login from container to all kubernetes nodes
cd /root/scripts/ || exit
bash -x ./configure_passwordless_login.sh

# Executing E2E tests
cd /mnt/gitsource/kubernetes || exit
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
export KUBE_SSH_USER="root"
export VSPHERE_KUBERNETES_CLUSTER="kubernetes"
export CLUSTER_DATASTORE="dscl1/sharedVmfs-0"
export VSPHERE_SPBM_POLICY_DS_CLUSTER=gold_cluster

export TEST_BEGIN="-------------------------------[ Starting the Test ]-------------------------------"
export TEST_END="-------------------------------[ End of the Test ]-------------------------------"

TESTS_SPEC_FILES[0]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_vsan_policy.go"
TESTS_SPEC_FILES[1]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_placement.go"
TESTS_SPEC_FILES[2]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_diskformat.go"
TESTS_SPEC_FILES[3]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_fstype.go"
TESTS_SPEC_FILES[4]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/pvc_label_selector.go"
TESTS_SPEC_FILES[5]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/persistent_volumes-vsphere.go"
TESTS_SPEC_FILES[6]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/pv_reclaimpolicy.go"
TESTS_SPEC_FILES[7]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/volumes.go"
TESTS_SPEC_FILES[8]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_ops_storm.go"
TESTS_SPEC_FILES[9]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_datastore.go"
TESTS_SPEC_FILES[10]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_cluster_ds.go"
TESTS_SPEC_FILES[11]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_disksize.go"
TESTS_SPEC_FILES[12]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_statefulsets.go"
TESTS_SPEC_FILES[13]="https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/vsphere_volume_master_restart.go"

TEST_SPEC_GINKGO_FOCUS[0]="--ginkgo.focus=Storage\sPolicy\sBased\sVolume\sProvisioning"
TEST_SPEC_GINKGO_FOCUS[1]="--ginkgo.focus=Volume\sPlacement"
TEST_SPEC_GINKGO_FOCUS[2]="--ginkgo.focus=Volume\sDisk\sFormat"
TEST_SPEC_GINKGO_FOCUS[3]="--ginkgo.focus=Volume\sFStype"
TEST_SPEC_GINKGO_FOCUS[4]="--ginkgo.focus=Selector-Label\sVolume\sBinding:vsphere"
TEST_SPEC_GINKGO_FOCUS[5]="--ginkgo.focus=PersistentVolumes:vsphere"
TEST_SPEC_GINKGO_FOCUS[6]="--ginkgo.focus=persistentvolumereclaim"
TEST_SPEC_GINKGO_FOCUS[7]="--ginkgo.focus=Volumes\svsphere"
TEST_SPEC_GINKGO_FOCUS[8]="--ginkgo.focus=Volume\sOperations\sStorm"
TEST_SPEC_GINKGO_FOCUS[9]="--ginkgo.focus=Volume\sProvisioning\son\sDatastore"
TEST_SPEC_GINKGO_FOCUS[10]="--ginkgo.focus=Volume\sProvisioning\sOn\sClustered\sDatastore"
TEST_SPEC_GINKGO_FOCUS[11]="--ginkgo.focus=Volume\sDisk\sSize"
TEST_SPEC_GINKGO_FOCUS[12]="--ginkgo.focus=vsphere\sstatefulset"
TEST_SPEC_GINKGO_FOCUS[13]="--ginkgo.focus=Volume\sAttach\sVerify"

function runTest(){
	GINKGO_FOCUS=$1
    echo "test" $GINKGO_FOCUS
	go run hack/e2e.go --check-version-skew=false --v --test --test_args="${GINKGO_FOCUS}"
    return $?
}

TEST_RESULT=()
for i in "${!TESTS_SPEC_FILES[@]}"; do
  printf "Running: %s\n" "${TESTS_SPEC_FILES[$i]}"
  echo $TEST_BEGIN
  runTest "${TEST_SPEC_GINKGO_FOCUS[$i]}"
  TEST_RESULT+=("$?")
  echo $TEST_END
done

# printing kubeconfig.json and cluster status
cat /mnt/gitsource/kubernetes-anywhere-config/kubeconfig.json
kubectl cluster-info
kubectl get nodes
set +x
echo "--------------------- Summary -----------------------"
for i in "${!TESTS_SPEC_FILES[@]}"; do
	if [ ${TEST_RESULT[$i]} != 0 ]
    then
    	printf "One or more test specs Failed from: %s\n" "${TESTS_SPEC_FILES[$i]}"
    else
    	printf "All test specs Passed from: %s\n" "${TESTS_SPEC_FILES[$i]}"
    fi
done
