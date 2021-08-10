package v1alpha1activemqartemissecurity

import (
	"context"

	brokerv1alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	v2alpha5 "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha5/activemqartemis"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/random"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_v1alpha1activemqartemissecurity")

//var namespacedNameToAddressName = make(map[types.NamespacedName]brokerv1alpha1.ActiveMQArtemisSecurity)

// Add creates a new ActiveMQArtemisSecurity Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileActiveMQArtemisSecurity{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("v1alpha1activemqartemissecurity-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ActiveMQArtemisSecurity
	err = c.Watch(&source.Kind{Type: &brokerv1alpha1.ActiveMQArtemisSecurity{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileActiveMQArtemisSecurity{}

type ReconcileActiveMQArtemisSecurity struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileActiveMQArtemisSecurity) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ActiveMQArtemisSecurity")

	instance := &brokerv1alpha1.ActiveMQArtemisSecurity{}

	if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			//unregister the CR
			v2alpha5.RemoveBrokerConfigHandler(request.NamespacedName)
			// Setting err to nil to prevent requeue
			err = nil
		} else {
			log.Error(err, "Reconcile errored thats not IsNotFound, requeuing request", "Request Namespace", request.Namespace, "Request Name", request.Name)
		}

		return reconcile.Result{}, err
	}

	reqLogger.Info("Fetched instance", "the instance", instance)
	v2alpha5.AddBrokerConfigHandler(request.NamespacedName, &ActiveMQArtemisSecurityConfigHandler{
		instance,
		request.NamespacedName,
		r,
	})
	return reconcile.Result{}, nil
}

type ActiveMQArtemisSecurityConfigHandler struct {
	SecurityCR     *brokerv1alpha1.ActiveMQArtemisSecurity
	NamespacedName types.NamespacedName
	owner          *ReconcileActiveMQArtemisSecurity
}

func (r *ActiveMQArtemisSecurityConfigHandler) processCrPasswords() *brokerv1alpha1.ActiveMQArtemisSecurity {
	result := r.SecurityCR.DeepCopy()

	if len(result.Spec.LoginModules.PropertiesLoginModules) > 0 {
		for i, pm := range result.Spec.LoginModules.PropertiesLoginModules {
			if len(pm.Users) > 0 {
				for j, user := range pm.Users {
					if user.Password == nil {
						result.Spec.LoginModules.PropertiesLoginModules[i].Users[j].Password = r.getPassword("security-properties-"+pm.Name, user.Name)
					}
					//Debug only. Dont log password in final commit!
					log.Info("==set prop login module password", "login module", pm.Name, "user", user.Name, "password", user.Password)
				}
			}
		}
	}

	if len(result.Spec.LoginModules.KeycloakLoginModules) > 0 {
		for _, pm := range result.Spec.LoginModules.KeycloakLoginModules {
			keycloakSecretName := "security-keycloak-" + pm.Name
			if pm.Configuration.ClientKeyStore != nil {
				if pm.Configuration.ClientKeyPassword == nil {
					pm.Configuration.ClientKeyPassword = r.getPassword(keycloakSecretName, "client-key-password")
				}
				if pm.Configuration.ClientKeyStorePassword == nil {
					pm.Configuration.ClientKeyStorePassword = r.getPassword(keycloakSecretName, "client-key-store-password")
				}
			}
			if pm.Configuration.TrustStore != nil {
				if pm.Configuration.TrustStorePassword == nil {
					pm.Configuration.TrustStorePassword = r.getPassword(keycloakSecretName, "trust-store-password")
				}
			}
			//need to process pm.Configuration.Credentials too. later.
			if len(pm.Configuration.Credentials) > 0 {
				for i, kv := range pm.Configuration.Credentials {
					if kv.Value == nil {
						pm.Configuration.Credentials[i].Value = r.getPassword(keycloakSecretName, "credentials-"+kv.Key)
					}
				}
			}
		}
	}
	return result
}

//retrive value from secret, generate value if not exist.
func (r *ActiveMQArtemisSecurityConfigHandler) getPassword(secretName string, key string) *string {
	//check if the secret exists.
	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: r.NamespacedName.Namespace,
	}
	// Attempt to retrieve the secret
	stringDataMap := make(map[string]string)

	secretDefinition := secrets.NewSecret(namespacedName, secretName, stringDataMap)

	if err := resources.Retrieve(namespacedName, r.owner.client, secretDefinition); err != nil {
		if errors.IsNotFound(err) {
			//create the secret
			resources.Create(r.SecurityCR, namespacedName, r.owner.client, r.owner.scheme, secretDefinition)
		}
	} else {
		log.Info("Found secret " + secretName)

		if elem, ok := secretDefinition.Data[key]; ok {
			//the value exists
			value := string(elem)
			return &value
		}
	}
	//now need generate value
	value := random.GenerateRandomString(8)
	//update the secret
	if secretDefinition.Data == nil {
		secretDefinition.Data = make(map[string][]byte)
	}
	secretDefinition.Data[key] = []byte(value)
	log.Info("Updating secret", "secret", namespacedName.Name)
	if err := resources.Update(namespacedName, r.owner.client, secretDefinition); err != nil {
		log.Error(err, "failed to update secret", "secret", secretName)
	}
	return &value
}

func (r *ActiveMQArtemisSecurityConfigHandler) Config(initContainers []corev1.Container, outputDirRoot string, yacfgProfileVersion string, yacfgProfileName string) (value []string) {
	log.Info("Reconciling security", "cr", r.SecurityCR)
	result := r.processCrPasswords()
	outputDir := outputDirRoot + "/security"
	var configCmds = []string{"echo \"making dir " + outputDir + "\"", "mkdir -p " + outputDir}
	filePath := outputDir + "/security-config.yaml"
	cmdPersistCRAsYaml, err := r.persistCR(filePath, result)
	if err != nil {
		log.Error(err, "Error marshalling security CR", "cr", r.SecurityCR)
		return nil
	}
	log.Info("get the command", "value", cmdPersistCRAsYaml)
	configCmds = append(configCmds, cmdPersistCRAsYaml)
	configCmds = append(configCmds, "/opt/amq-broker/script/cfg/config-security.sh")
	envVarName := "SECURITY_CFG_YAML"
	envVar := corev1.EnvVar{
		envVarName,
		filePath,
		nil,
	}
	environments.Create(initContainers, &envVar)

	envVarName = "YACFG_PROFILE_VERSION"
	envVar = corev1.EnvVar{
		envVarName,
		yacfgProfileVersion,
		nil,
	}
	environments.Create(initContainers, &envVar)

	envVarName = "YACFG_PROFILE_NAME"
	envVar = corev1.EnvVar{
		envVarName,
		yacfgProfileName,
		nil,
	}
	environments.Create(initContainers, &envVar)

	log.Info("returning config cmds", "value", configCmds)
	return configCmds
}

func (r *ActiveMQArtemisSecurityConfigHandler) persistCR(filePath string, cr *brokerv1alpha1.ActiveMQArtemisSecurity) (value string, err error) {

	data, err := yaml.Marshal(cr)
	if err != nil {
		return "", err
	}
	return "echo \"" + string(data) + "\" > " + filePath, nil
}
