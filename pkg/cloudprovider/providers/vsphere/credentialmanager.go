package vsphere

import (
	"fmt"
	"github.com/golang/glog"
	"gopkg.in/gcfg.v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/listers/core/v1"
	"net/http"
	"sync"
)

type SecretCache struct {
	cacheLock     sync.Mutex
	VirtualCenter map[string]*Credential
	Secret        *corev1.Secret
}

type Credential struct {
	User     string `gcfg:"user"`
	Password string `gcfg:"password"`
}

type SecretCredentialManager struct {
	SecretName      string
	SecretNamespace string
	SecretLister    v1.SecretLister
	Cache           *SecretCache
}

func (secretCredentialManager *SecretCredentialManager) GetCredential(server string) (*Credential, error) {
	err := secretCredentialManager.updateCredentialsMap()
	// Handle secret deletion
	if err != nil {
		statusErr, ok := err.(*apierrors.StatusError)
		if ok && statusErr.ErrStatus.Code != http.StatusNotFound || !ok {
			return nil, err
		}
		glog.Warningf("secret %q not found in namespace %q", secretCredentialManager.SecretName, secretCredentialManager.SecretNamespace)
	}
	// Cases:
	// 1. Secret Deleted finding credentials from cache
	// 2. Secret Not Added at a first place will return error
	// 3. Secret Added but not for asked vCenter Server
	credential, found := secretCredentialManager.Cache.GetCredential(server)
	if !found {
		return nil, fmt.Errorf("credentials not found for server %q", server)
	}
	return &credential, nil
}

func (secretCredentialManager *SecretCredentialManager) updateCredentialsMap() error {
	if secretCredentialManager.SecretLister == nil {
		return fmt.Errorf("SecretLister is not initialized")
	}
	secret, err := secretCredentialManager.SecretLister.Secrets(secretCredentialManager.SecretNamespace).Get(secretCredentialManager.SecretName)
	if err != nil {
		glog.Errorf("Cannot get secret %s in namespace %s. error: %q", secretCredentialManager.SecretName, secretCredentialManager.SecretNamespace, err)
		return err
	}
	cacheSecret := secretCredentialManager.Cache.GetSecret()
	if cacheSecret != nil &&
		cacheSecret.GetResourceVersion() == secret.GetResourceVersion() {
		glog.V(4).Infof("VCP SecretCredentialManager: Secret %q will not be updated in cache. Since, secrets have same resource version %q", secretCredentialManager.SecretName, cacheSecret.GetResourceVersion())
		return nil
	}
	secretCredentialManager.Cache.UpdateSecret(secret)
	return secretCredentialManager.Cache.parseSecret()
}

func (cache *SecretCache) GetSecret() *corev1.Secret {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	return cache.Secret
}

func (cache *SecretCache) UpdateSecret(secret *corev1.Secret) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	cache.Secret = secret
}

func (cache *SecretCache) GetCredential(server string) (Credential, bool) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	credential, found := cache.VirtualCenter[server]
	return *credential, found
}

func (cache *SecretCache) parseSecret() error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	confData, found := cache.Secret.Data["vsphere.conf"]
	if !found {
		return fmt.Errorf("vsphere.conf not found in cache")
	}

	glog.Errorf("Data %+v, ConfData %+v, String Version %q", cache.Secret.Data["vsphere.conf"], confData, string(confData))
	return gcfg.ReadStringInto(cache, string(confData))
}
