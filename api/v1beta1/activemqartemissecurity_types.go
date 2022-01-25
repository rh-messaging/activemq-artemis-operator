/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ActiveMQArtemisSecuritySpec defines the desired state of ActiveMQArtemisSecurity
type ActiveMQArtemisSecuritySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	LoginModules     LoginModulesType     `json:"loginModules"`
	SecurityDomains  SecurityDomainsType  `json:"securityDomains"`
	SecuritySettings SecuritySettingsType `json:"securitySettings"`
	// Apply this security config to the broker crs in the current namespace. A value of * or empty string means applying to all broker crs. Default apply to all broker crs
	ApplyToCrNames []string `json:"applyToCrNames,omitempty"`
}

type LoginModulesType struct {
	PropertiesLoginModules []PropertiesLoginModuleType `json:"propertiesLoginModules,omitempty"`
	GuestLoginModules      []GuestLoginModuleType      `json:"guestLoginModules,omitempty"`
	KeycloakLoginModules   []KeycloakLoginModuleType   `json:"keycloakLoginModules,omitempty"`
}

type PropertiesLoginModuleType struct {
	// Name for PropertiesLoginModule
	Name  string     `json:"name,omitempty"`
	Users []UserType `json:"users,omitempty"`
}

type UserType struct {
	// User name to be defined in properties login module
	Name string `json:"name,omitempty"`
	// Password to be defined in properties login module
	Password *string `json:"password,omitempty"`
	// Roles to be defined in properties login module
	Roles []string `json:"roles,omitempty"`
}

type GuestLoginModuleType struct {
	// Name for GuestLoginModule
	Name string `json:"name,omitempty"`
	// The guest user name
	GuestUser *string `json:"guestUser,omitempty"`
	// The guest user role
	GuestRole *string `json:"guestRole,omitempty"`
}

type KeycloakLoginModuleType struct {
	// Name for KeycloakLoginModule
	Name string `json:"name,omitempty"`
	// Type of KeycloakLoginModule directAccess or bearerToken
	ModuleType    *string                         `json:"moduleType,omitempty"`
	Configuration KeycloakModuleConfigurationType `json:"configuration,omitempty"`
}

type KeycloakModuleConfigurationType struct {
	// Realm for KeycloakLoginModule
	Realm *string `json:"realm,omitempty"`
	// Public key for the realm
	RealmPublicKey *string `json:"realmPublicKey,omitempty"`
	// URL of the keycloak authentication server
	AuthServerUrl *string `json:"authServerUrl,omitempty"`
	// How SSL is required
	SslRequired *string `json:"sslRequired,omitempty"`
	// Resource Name
	Resource *string `json:"resource,omitempty"`
	// If it is public client
	PublicClient *bool          `json:"publicClient,omitempty"`
	Credentials  []KeyValueType `json:"credentials,omitempty"`
	// If to use resource role mappings
	UseResourceRoleMappings *bool `json:"useResourceRoleMappings,omitempty"`
	// If to enable CORS
	EnableCors *bool `json:"enableCors,omitempty"`
	// CORS max age
	CorsMaxAge *int64 `json:"corsMaxAge,omitempty"`
	// CORS allowed methods
	CorsAllowedMethods *string `json:"corsAllowedMethods,omitempty"`
	// CORS allowed headers
	CorsAllowedHeaders *string `json:"corsAllowedHeaders,omitempty"`
	// CORS exposed headers
	CorsExposedHeaders *string `json:"corsExposedHeaders,omitempty"`
	// If to expose access token
	ExposeToken *bool `json:"exposeToken,omitempty"`
	// If only verify bearer token
	BearerOnly *bool `json:"bearerOnly,omitempty"`
	// If auto-detect bearer token only
	AutoDetectBearerOnly *bool `json:"autoDetectBearerOnly,omitempty"`
	// Size of the connection pool
	ConnectionPoolSize *int64 `json:"connectionPoolSize,omitempty"`
	// If to allow any host name
	AllowAnyHostName *bool `json:"allowAnyHostName,omitempty"`
	// If to disable trust manager
	DisableTrustManager *bool `json:"disableTrustManager,omitempty"`
	// Path of a trust store
	TrustStore *string `json:"trustStore,omitempty"`
	// Truststore password
	TrustStorePassword *string `json:"trustStorePassword,omitempty"`
	// Path of a client keystore
	ClientKeyStore *string `json:"clientKeyStore,omitempty"`
	// Client keystore password
	ClientKeyStorePassword *string `json:"clientKeyStorePassword,omitempty"`
	// Client key password
	ClientKeyPassword *string `json:"clientKeyPassword,omitempty"`
	// If always refresh token
	AlwaysRefreshToken *bool `json:"alwaysRefreshToken,omitempty"`
	// If register node at startup
	RegisterNodeAtStartup *bool `json:"registerNodeAtStartup,omitempty"`
	// Period for re-registering node
	RegisterNodePeriod *int64 `json:"registerNodePeriod,omitempty"`
	// Type of token store. session or cookie
	TokenStore *string `json:"tokenStore,omitempty"`
	// Cookie path for a cookie store
	TokenCookiePath *string `json:"tokenCookiePath,omitempty"`
	// OpenID Connect ID Token attribute to populate the UserPrincipal name with
	PrincipalAttribute *string `json:"principalAttribute,omitempty"`
	// The proxy URL
	ProxyUrl *string `json:"proxyUrl,omitempty"`
	// If not to change session id on a successful login
	TurnOffChangeSessionIdOnLogin *bool `json:"turnOffChangeSessionIdOnLogin,omitempty"`
	// Minimum time to refresh an active access token
	TokenMinimumTimeToLive *int64 `json:"tokenMinimumTimeToLive,omitempty"`
	// Minimum interval between two requests to Keycloak to retrieve new public keys
	MinTimeBetweenJwksRequests *int64 `json:"minTimeBetweenJwksRequests,omitempty"`
	// Maximum interval between two requests to Keycloak to retrieve new public keys
	PublicKeyCacheTtl *int64 `json:"publicKeyCacheTtl,omitempty"`
	// Whether to turn off processing of the access_token query parameter for bearer token processing
	IgnoreOauthQueryParameter *bool `json:"ignoreOauthQueryParameter,omitempty"`
	// Verify whether the token contains this client name (resource) as an audience
	VerifyTokenAudience *bool `json:"verifyTokenAudience,omitempty"`
	// Whether to support basic authentication
	EnableBasicAuth *bool `json:"enableBasicAuth"`
	// The confidential port used by the Keycloak server for secure connections over SSL/TLS
	ConfidentialPort *int32 `json:"confidentialPort,omitempty"`
	// The regular expression to which the Redirect URI is to be matched
	// value is the replacement String
	RedirectRewriteRules []KeyValueType `json:"redirectRewriteRules,omitempty"`
	// The OAuth2 scope parameter for DirectAccessGrantsLoginModule
	Scope *string `json:"scope,omitempty"`
}

type KeyValueType struct {
	// The credentials key
	Key string `json:"key,omitempty"`
	// The credentials value
	Value *string `json:"value,omitempty"`
}

type SecurityDomainsType struct {
	BrokerDomain  BrokerDomainType `json:"brokerDomain,omitempty"`
	ConsoleDomain BrokerDomainType `json:"consoleDomain,omitempty"`
}

type BrokerDomainType struct {
	// Name for the broker/console domain
	Name         *string                    `json:"name,omitempty"`
	LoginModules []LoginModuleReferenceType `json:"loginModules,omitempty"`
}

type LoginModuleReferenceType struct {
	// Name for login modules for broker/console domain
	Name *string `json:"name,omitempty"`
	// Flag of login modules for broker/console domain
	Flag *string `json:"flag,omitempty"`
	// Debug option of login modules for broker/console domain
	Debug *bool `json:"debug,omitempty"`
	// Reload option of login modules for broker/console domain
	Reload *bool `json:"reload,omitempty"`
}

type SecuritySettingsType struct {
	Broker     []BrokerSecuritySettingType    `json:"broker,omitempty"`
	Management ManagementSecuritySettingsType `json:"management,omitempty"`
}

type BrokerSecuritySettingType struct {
	// The address match pattern of a security setting
	Match       string           `json:"match,omitempty"`
	Permissions []PermissionType `json:"permissions,omitempty"`
}

type PermissionType struct {
	// The operation type of a security setting
	OperationType string `json:"operationType"`
	// The roles of a security setting
	Roles []string `json:"roles,omitempty"`
}

type ManagementSecuritySettingsType struct {
	// The roles allowed to login hawtio
	HawtioRoles   []string                `json:"hawtioRoles,omitempty"`
	Connector     ConnectorConfigType     `json:"connector,omitempty"`
	Authorisation AuthorisationConfigType `json:"authorisation,omitempty"`
}

type ConnectorConfigType struct {
	// The connector host for connecting to management
	Host *string `json:"host,omitempty"`
	// The connector port for connecting to management
	Port *int32 `json:"port,omitempty"`
	// The RMI registry port for management
	RmiRegistryPort *int32 `json:"rmiRegistryPort,omitempty"`
	// The JMX realm of management
	JmxRealm *string `json:"jmxRealm,omitempty"`
	// The JMX object name of management
	ObjectName *string `json:"objectName,omitempty"`
	// The management authentication type
	AuthenticatorType *string `json:"authenticatorType,omitempty"`
	// Whether management connection is secured
	Secured *bool `json:"secured,omitempty"`
	// The keystore provider for management connector
	KeyStoreProvider *string `json:"keyStoreProvider,omitempty"`
	// The keystore path for management connector
	KeyStorePath *string `json:"keyStorePath,omitempty"`
	// The keystore password for management connector
	KeyStorePassword *string `json:"keyStorePassword,omitempty"`
	// The truststore provider for management connector
	TrustStoreProvider *string `json:"trustStoreProvider,omitempty"`
	// The truststore path for management connector
	TrustStorePath *string `json:"trustStorePath,omitempty"`
	// The truststore password for management connector
	TrustStorePassword *string `json:"trustStorePassword,omitempty"`
	// The password codec for management connector
	PasswordCodec *string `json:"passwordCodec,omitempty"`
}

type AuthorisationConfigType struct {
	AllowedList   []AllowedListEntryType `json:"allowedList,omitempty"`
	DefaultAccess []DefaultAccessType    `json:"defaultAccess,omitempty"`
	RoleAccess    []RoleAccessType       `json:"roleAccess,omitempty"`
}

type AllowedListEntryType struct {
	// The domain of allowedList
	Domain *string `json:"domain,omitempty"`
	// The key of allowedList
	Key *string `json:"key,omitempty"`
}

type DefaultAccessType struct {
	// The method of defaultAccess/roleAccess List
	Method *string `json:"method,omitempty"`
	// The roles of defaultAccess/roleAccess List
	Roles []string `json:"roles,omitempty"`
}

type RoleAccessType struct {
	// The domain of roleAccess List
	Domain *string `json:"domain,omitempty"`
	// The key of roleAccess List
	Key        *string             `json:"key,omitempty"`
	AccessList []DefaultAccessType `json:"accessList,omitempty"`
}

// ActiveMQArtemisSecurityStatus defines the observed state of ActiveMQArtemisSecurity
type ActiveMQArtemisSecurityStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// ActiveMQArtemisSecurity is the Schema for the activemqartemissecurities API
type ActiveMQArtemisSecurity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActiveMQArtemisSecuritySpec   `json:"spec,omitempty"`
	Status ActiveMQArtemisSecurityStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ActiveMQArtemisSecurityList contains a list of ActiveMQArtemisSecurity
type ActiveMQArtemisSecurityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActiveMQArtemisSecurity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActiveMQArtemisSecurity{}, &ActiveMQArtemisSecurityList{})
}

func (r *ActiveMQArtemisSecurity) Hub() {
}
