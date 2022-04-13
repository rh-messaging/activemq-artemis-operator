package v1alpha1activemqartemissecurity

import (
	"context"
	"reflect"

	brokerv1alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	v2alpha5 "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha5/activemqartemis"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/lsrcrs"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/random"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
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
			//clean the CR
			lsrcrs.DeleteLastSuccessfulReconciledCR(request.NamespacedName, "security", getLabels(instance), r.client)
		} else {
			log.Error(err, "Reconcile errored thats not IsNotFound, requeuing request", "Request Namespace", request.Namespace, "Request Name", request.Name)
		}

		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: common.GetReconcileResyncPeriod()}, nil
	}

	toReconcile := true
	newHandler := &ActiveMQArtemisSecurityConfigHandler{
		instance,
		request.NamespacedName,
		r,
	}

	if securityHandler := v2alpha5.GetBrokerConfigHandler(request.NamespacedName); securityHandler == nil {
		log.Info("Operator doesn't have the security handler, try retrive it from secret")
		if existingHandler := lsrcrs.RetrieveLastSuccessfulReconciledCR(request.NamespacedName, "security", r.client, getLabels(instance)); existingHandler != nil {
			//compare resource version
			if existingHandler.Checksum == instance.ResourceVersion {
				log.V(1).Info("The incoming security CR is identical to stored CR, no reconcile")
				toReconcile = false
			}
		}
	} else {
		log.V(1).Info("We have an existing handler")
		if reflect.DeepEqual(securityHandler, newHandler) {
			log.Info("Will not reconcile the same security config")
			return reconcile.Result{RequeueAfter: common.GetReconcileResyncPeriod()}, nil
		}
	}

	if err := v2alpha5.AddBrokerConfigHandler(request.NamespacedName, newHandler, toReconcile); err != nil {
		log.Error(err, "failed to config security cr", "request", request.NamespacedName)
		return reconcile.Result{}, nil
	}
	//persist the CR
	crstr, merr := common.ToJson(instance)
	if merr != nil {
		log.Error(merr, "failed to marshal cr")
	}

	lsrcrs.StoreLastSuccessfulReconciledCR(instance, instance.Name, instance.Namespace, "security",
		crstr, "", instance.ResourceVersion, getLabels(instance), r.client, r.scheme)

	return reconcile.Result{RequeueAfter: common.GetReconcileResyncPeriod()}, nil
}

type ActiveMQArtemisSecurityConfigHandler struct {
	SecurityCR     *brokerv1alpha1.ActiveMQArtemisSecurity
	NamespacedName types.NamespacedName
	owner          *ReconcileActiveMQArtemisSecurity
}

func getLabels(cr *brokerv1alpha1.ActiveMQArtemisSecurity) map[string]string {
	labelBuilder := selectors.LabelerData{}
	labelBuilder.Base(cr.Name).Suffix("sec").Generate()
	return labelBuilder.Labels()
}

func (r *ActiveMQArtemisSecurityConfigHandler) IsApplicableFor(brokerNamespacedName types.NamespacedName) bool {
	reqLogger := log.WithValues("IsApplicableFor", brokerNamespacedName)
	applyTo := r.SecurityCR.Spec.ApplyToCrNames
	reqLogger.V(1).Info("applyTo", "len", len(applyTo), "sec", r.SecurityCR.Spec)

	//currently security doesnt apply to other namespaces than its own
	if r.NamespacedName.Namespace != brokerNamespacedName.Namespace {
		reqLogger.V(1).Info("this security cr is not applicable for broker because it's not in my namespace")
		return false
	}

	if len(applyTo) == 0 {
		reqLogger.V(1).Info("this security cr is applicable for broker because no applyTo is configured")
		return true
	}
	for _, crName := range applyTo {
		reqLogger.V(1).Info("Going through applyTo", "crName", crName)
		if crName == "*" || crName == "" || crName == brokerNamespacedName.Name {
			reqLogger.V(1).Info("this security cr is applicable for broker as it's either match-all or match name")
			return true
		}
	}
	reqLogger.V(1).Info("all applyToCrNames checked, no match. Not applicable")
	return false
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

func (r *ActiveMQArtemisSecurityConfigHandler) GetDefaultLabels() map[string]string {
	defaultLabelData := selectors.LabelerData{}
	defaultLabelData.Base(r.SecurityCR.Name).Suffix("app").Generate()
	return defaultLabelData.Labels()

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

	secretDefinition := secrets.NewSecret(namespacedName, secretName, stringDataMap, r.GetDefaultLabels())

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
