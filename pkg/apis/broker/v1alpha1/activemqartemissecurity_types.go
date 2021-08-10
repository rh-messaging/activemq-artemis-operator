package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisSecuritySpec defines the desired state of ActiveMQArtemisSecurity
// +k8s:openapi-gen=true
type ActiveMQArtemisSecuritySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	LoginModules     LoginModulesType     `json:"loginModules"`
	SecurityDomains  SecurityDomainsType  `json:"securityDomains"`
	SecuritySettings SecuritySettingsType `json:"securitySettings"`
}

type LoginModulesType struct {
	PropertiesLoginModules []PropertiesLoginModuleType `json:"propertiesLoginModules,omitempty"`
	GuestLoginModules      []GuestLoginModuleType      `json:"guestLoginModules,omitempty"`
	KeycloakLoginModules   []KeycloakLoginModuleType   `json:"keycloakLoginModules,omitempty"`
}

type PropertiesLoginModuleType struct {
	Name  string     `json:"name,omitempty"`
	Users []UserType `json:"users,omitempty"`
}

type UserType struct {
	Name     string   `json:"name,omitempty"`
	Password *string  `json:"password,omitempty"`
	Roles    []string `json:"roles,omitempty"`
}

type GuestLoginModuleType struct {
	Name      string  `json:"name,omitempty"`
	GuestUser *string `json:"guestUser,omitempty"`
	GuestRole *string `json:"guestRole,omitempty"`
}

type KeycloakLoginModuleType struct {
	Name          string                          `json:"name,omitempty"`
	ModuleType    *string                         `json:"moduleType,omitempty"`
	Configuration KeycloakModuleConfigurationType `json:"configuration,omitempty"`
}

type KeycloakModuleConfigurationType struct {
	Realm                         *string        `json:"realm,omitempty"`
	RealmPublicKey                *string        `json:"realmPublicKey,omitempty"`
	AuthServerUrl                 *string        `json:"authServerUrl,omitempty"`
	SslRequired                   *string        `json:"sslRequired,omitempty"`
	Resource                      *string        `json:"resource,omitempty"`
	PublicClient                  *bool          `json:"publicClient,omitempty"`
	Credentials                   []KeyValueType `json:"credentials,omitempty"`
	UseResourceRoleMappings       *bool          `json:"useResourceRoleMappings,omitempty"`
	EnableCors                    *bool          `json:"enableCors,omitempty"`
	CorsMaxAge                    *int64         `json:"corsMaxAge,omitempty"`
	CorsAllowedMethods            *string        `json:"corsAllowedMethods,omitempty"`
	CorsAllowedHeaders            *string        `json:"corsAllowedHeaders,omitempty"`
	CorsExposedHeaders            *string        `json:"corsExposedHeaders,omitempty"`
	ExposeToken                   *bool          `json:"exposeToken,omitempty"`
	BearerOnly                    *bool          `json:"bearerOnly,omitempty"`
	AutoDetectBearerOnly          *bool          `json:"autoDetectBearerOnly,omitempty"`
	ConnectionPoolSize            *int64         `json:"connectionPoolSize,omitempty"`
	AllowAnyHostName              *bool          `json:"allowAnyHostName,omitempty"`
	DisableTrustManager           *bool          `json:"disableTrustManager,omitempty"`
	TrustStore                    *string        `json:"trustStore,omitempty"`
	TrustStorePassword            *string        `json:"trustStorePassword,omitempty"`
	ClientKeyStore                *string        `json:"clientKeyStore,omitempty"`
	ClientKeyStorePassword        *string        `json:"clientKeyStorePassword,omitempty"`
	ClientKeyPassword             *string        `json:"clientKeyPassword,omitempty"`
	AlwaysRefreshToken            *bool          `json:"alwaysRefreshToken,omitempty"`
	RegisterNodeAtStartup         *bool          `json:"registerNodeAtStartup,omitempty"`
	RegisterNodePeriod            *int64         `json:"registerNodePeriod,omitempty"`
	TokenStore                    *string        `json:"tokenStore,omitempty"`
	TokenCookiePath               *string        `json:"tokenCookiePath,omitempty"`
	PrincipalAttribute            *string        `json:"principalAttribute,omitempty"`
	ProxyUrl                      *string        `json:"proxyUrl,omitempty"`
	TurnOffChangeSessionIdOnLogin *bool          `json:"turnOffChangeSessionIdOnLogin,omitempty"`
	TokenMinimumTimeToLive        *int64         `json:"tokenMinimumTimeToLive,omitempty"`
	MinTimeBetweenJwksRequests    *int64         `json:"minTimeBetweenJwksRequests,omitempty"`
	PublicKeyCacheTtl             *int64         `json:"publicKeyCacheTtl,omitempty"`
	IgnoreOauthQueryParameter     *bool          `json:"ignoreOauthQueryParameter,omitempty"`
	VerifyTokenAudience           *bool          `json:"verifyTokenAudience,omitempty"`
	EnableBasicAuth               *bool          `json:"enableBasicAuth"`
	ConfidentialPort              *int32         `json:"confidentialPort,omitempty"`
	RedirectRewriteRules          []KeyValueType `json:"redirectRewriteRules,omitempty"`
	Scope                         *string        `json:"scope,omitempty"`
}

type KeyValueType struct {
	Key   string  `json:"key,omitempty"`
	Value *string `json:"value,omitempty"`
}

type SecurityDomainsType struct {
	BrokerDomain  BrokerDomainType `json:"brokerDomain,omitempty"`
	ConsoleDomain BrokerDomainType `json:"consoleDomain,omitempty"`
}

type BrokerDomainType struct {
	Name         *string                    `json:"name,omitempty"`
	LoginModules []LoginModuleReferenceType `json:"loginModules,omitempty"`
}

type LoginModuleReferenceType struct {
	Name   *string `json:"name,omitempty"`
	Flag   *string `json:"flag,omitempty"`
	Debug  *bool   `json:"debug,omitempty"`
	Reload *bool   `json:"reload,omitempty"`
}

type SecuritySettingsType struct {
	Broker     []BrokerSecuritySettingType    `json:"broker,omitempty"`
	Management ManagementSecuritySettingsType `json:"management,omitempty"`
}

type BrokerSecuritySettingType struct {
	Match       string           `json:"match,omitempty"`
	Permissions []PermissionType `json:"permissions,omitempty"`
}

type PermissionType struct {
	OperationType string   `json:"operationType"`
	Roles         []string `json:"roles,omitempty"`
}

type ManagementSecuritySettingsType struct {
	HawtioRoles   []string                `json:"hawtioRoles,omitempty"`
	Connector     ConnectorConfigType     `json:"connector,omitempty"`
	Authorisation AuthorisationConfigType `json:"authorisation,omitempty"`
}

type ConnectorConfigType struct {
	Host               *string `json:"host,omitempty"`
	Port               *int32  `json:"port,omitempty"`
	RmiRegistryPort    *int32  `json:"rmiRegistryPort,omitempty"`
	JmxRealm           *string `json:"jmxRealm,omitempty"`
	ObjectName         *string `json:"objectName,omitempty"`
	AuthenticatorType  *string `json:"authenticatorType,omitempty"`
	Secured            *bool   `json:"secured,omitempty"`
	KeyStoreProvider   *string `json:"keyStoreProvider,omitempty"`
	KeyStorePath       *string `json:"keyStorePath,omitempty"`
	KeyStorePassword   *string `json:"keyStorePassword,omitempty"`
	TrustStoreProvider *string `json:"trustStoreProvider,omitempty"`
	TrustStorePath     *string `json:"trustStorePath,omitempty"`
	TrustStorePassword *string `json:"trustStorePassword,omitempty"`
	PasswordCodec      *string `json:"passwordCodec,omitempty"`
}

type AuthorisationConfigType struct {
	AllowedList   []AllowedListEntryType `json:"allowedList,omitempty"`
	DefaultAccess []DefaultAccessType    `json:"defaultAccess,omitempty"`
	RoleAccess    []RoleAccessType       `json:"roleAccess,omitempty"`
}

type AllowedListEntryType struct {
	Domain *string `json:"domain,omitempty"`
	Key    *string `json:"key,omitempty"`
}

type DefaultAccessType struct {
	Method *string  `json:"method,omitempty"`
	Roles  []string `json:"roles,omitempty"`
}

type RoleAccessType struct {
	Domain     *string             `json:"domain,omitempty"`
	Key        *string             `json:"key,omitempty"`
	AccessList []DefaultAccessType `json:"accessList,omitempty"`
}

// ActiveMQArtemisSecurityStatus defines the observed state of ActiveMQArtemisSecurity
// +k8s:openapi-gen=true
type ActiveMQArtemisSecurityStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemisSecurity is the Schema for the activemqartemissecurities API
// +k8s:openapi-gen=true
// +genclient
type ActiveMQArtemisSecurity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisSecuritySpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisSecurityStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveMQArtemisSecurityList contains a list of ActiveMQArtemisSecurity
type ActiveMQArtemisSecurityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemisSecurity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemisSecurity{}, &ActiveMQArtemisSecurityList{})
}
