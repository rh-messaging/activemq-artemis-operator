package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/blang/semver"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"strconv"

	oimagev1 "github.com/openshift/api/image/v1"
	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmversion "github.com/operator-framework/operator-lifecycle-manager/pkg/lib/version"
	api "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha2"
	activemqartemis "github.com/rh-messaging/activemq-artemis-operator/pkg/controller/broker/v2alpha2/activemqartemis"
	"github.com/rh-messaging/activemq-artemis-operator/tools/components"
	"github.com/rh-messaging/activemq-artemis-operator/tools/constants"
	"github.com/rh-messaging/activemq-artemis-operator/tools/util"
	"github.com/rh-messaging/activemq-artemis-operator/version"
	"github.com/tidwall/sjson"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	//"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

var (
	rh              = "Red Hat"
	maturity        = "stable"
	major, minor, _ = activemqartemis.MajorMinorMicro(activemqartemis.CurrentVersion)
	csv             = csvSetting{

		Name:         "activemq-artemis",
		DisplayName:  "ActiveMQ Artemis",
		OperatorName: "activemq-artemis-operator",
		CsvDir:       "activemq-artemis-operator",
		Registry:     "quay.io",
		Context:      "artemiscloud",
		ImageName:    "activemq-artemis-operator",
		Tag:          version.Version,
	}
)

func main() {

	imageShaMap := map[string]string{}

	operatorName := csv.Name + "-operator"
	templateStruct := &csvv1.ClusterServiceVersion{}
	templateStruct.SetGroupVersionKind(csvv1.SchemeGroupVersion.WithKind("ClusterServiceVersion"))
	csvStruct := &csvv1.ClusterServiceVersion{}
	strategySpec := &csvStrategySpec{}
	json.Unmarshal(csvStruct.Spec.InstallStrategy.StrategySpecRaw, strategySpec)

	templateStrategySpec := &csvStrategySpec{}
	deployment := components.GetDeployment(csv.OperatorName, csv.Registry, csv.Context, csv.ImageName, csv.Tag, "Always")
	templateStrategySpec.Deployments = append(templateStrategySpec.Deployments, []csvDeployments{{Name: csv.OperatorName, Spec: deployment.Spec}}...)
	role := components.GetRole(csv.OperatorName)
	templateStrategySpec.Permissions = append(templateStrategySpec.Permissions, []csvPermissions{{ServiceAccountName: deployment.Spec.Template.Spec.ServiceAccountName, Rules: role.Rules}}...)

	// Re-serialize deployments and permissions into csv strategy.
	updatedStrat, err := json.Marshal(templateStrategySpec)
	if err != nil {
		panic(err)
	}
	templateStruct.Spec.InstallStrategy.StrategySpecRaw = updatedStrat
	templateStruct.Spec.InstallStrategy.StrategyName = "deployment"
	csvVersionedName := operatorName + ".v" + version.Version
	templateStruct.Name = csvVersionedName
	templateStruct.Namespace = "placeholder"
	descrip := "ActiveMQ Artemis Operator provides the ability to deploy and manage stateful ActiveMQ Artemis broker clusters"
	repository := "https://github.com/rh-messaging/activemq-artemis-operator"
	examples := []string{"{\n          \"apiVersion\": \"broker.amq.io/v2alpha2\",\n          \"kind\": \"ActiveMQArtemis\",\n          \"metadata\": {\n             \"name\": \"ex-aao\",\n             \"application\": \"ex-aao-app\"\n          },\n          \"version\": \"7.6.0\",\n          \"spec\": {\n             \"deploymentPlan\": {\n                \"size\": 1,\n                \"image\": \"registry.redhat.io/amq7/amq-broker:7.6\",\n                \"requireLogin\": false,\n                \"persistenceEnabled\": false,\n                \"journalType\": \"nio\",\n                \"messageMigration\": false\n             },\n             \"console\": {\n                \"expose\": true\n             },\n             \"acceptors\": [\n                {\n                   \"name\": \"amqp\",\n                   \"protocols\": \"amqp\",\n                   \"port\": 5672,\n                   \"sslEnabled\": false,\n                   \"enabledCipherSuites\": \"SSL_RSA_WITH_RC4_128_SHA,SSL_DH_anon_WITH_3DES_EDE_CBC_SHA\",\n                   \"enabledProtocols\": \"TLSv1,TLSv1.1,TLSv1.2\",\n                   \"needClientAuth\": true,\n                   \"wantClientAuth\": true,\n                   \"verifyHost\": true,\n                   \"sslProvider\": \"JDK\",\n                   \"sniHost\": \"localhost\",\n                   \"expose\": true,\n                   \"anycastPrefix\": \"jms.topic.\",\n                   \"multicastPrefix\": \"/queue/\"\n                }\n             ],\n             \"connectors\": [\n                {\n                   \"name\": \"connector0\",\n                   \"host\": \"localhost\",\n                   \"port\": 22222,\n                   \"sslEnabled\": false,\n                   \"enabledCipherSuites\": \"SSL_RSA_WITH_RC4_128_SHA,SSL_DH_anon_WITH_3DES_EDE_CBC_SHA\",\n                   \"enabledProtocols\": \"TLSv1,TLSv1.1,TLSv1.2\",\n                   \"needClientAuth\": true,\n                   \"wantClientAuth\": true,\n                   \"verifyHost\": true,\n                   \"sslProvider\": \"JDK\",\n                   \"sniHost\": \"localhost\",\n                   \"expose\": true\n                }\n             ]\n          }\n       },\n      {\n          \"apiVersion\": \"broker.amq.io/v2alpha1\",\n          \"kind\": \"ActiveMQArtemisAddress\",\n          \"metadata\": {\n              \"name\": \"ex-aaoaddress\"\n          },\n          \"spec\": {\n              \"addressName\": \"myAddress0\",\n              \"queueName\": \"myQueue0\",\n              \"routingType\": \"anycast\"\n          }\n      },\n      {\n          \"apiVersion\": \"broker.amq.io/v2alpha1\",\n          \"kind\": \"ActiveMQArtemisScaledown\",\n          \"metadata\": {\n              \"name\": \"ex-aaoscaledown\"\n          },\n          \"spec\": {\n              \"localOnly\": \"true\"\n          }\n      }"}
	templateStruct.SetAnnotations(
		map[string]string{
			"createdAt":           time.Now().Format("2006-01-02 15:04:05"),
			"containerImage":      deployment.Spec.Template.Spec.Containers[0].Image,
			"description":         descrip,
			"categories":          "Streaming & Messaging",
			"certified":           "false",
			"capabilities":        "Seamless Upgrades",
			"repository":          repository,
			"support":             rh,
			"tectonic-visibility": "ocs",
			"alm-examples":        "[" + strings.Join(examples, ",") + "]",
		},
	)
	templateStruct.SetLabels(
		map[string]string{
			"operator-" + csv.Name:            "true",
			"operatorframework.io/arch.amd64": "supported",
			"operatorframework.io/arch.s390x": "supported",
		},
	)

	templateStruct.Spec.Keywords = []string{"activemqartemis", "operator"}
	var opVersion olmversion.OperatorVersion
	opVersion.Version = semver.MustParse(version.Version)
	templateStruct.Spec.Version = opVersion
	templateStruct.Spec.Replaces = operatorName + ".v" + version.PriorVersion
	templateStruct.Spec.Description = descrip
	templateStruct.Spec.DisplayName = csv.DisplayName
	templateStruct.Spec.Maturity = maturity
	templateStruct.Spec.Maintainers = []csvv1.Maintainer{{Name: rh, Email: "rkieley@redhat.com"}}
	templateStruct.Spec.Provider = csvv1.AppLink{Name: rh}
	templateStruct.Spec.Links = []csvv1.AppLink{
		{Name: "Product Page", URL: "https://access.redhat.com/products/red-hat-amq/"},
		{Name: "Documentation", URL: "https://access.redhat.com/documentation/en-us/red_hat_amq/" + major + "." + minor + "/html/migrating_to_red_hat_amq_7/index"},
	}

	templateStruct.Spec.Icon = []csvv1.Icon{
		{
			Data:      "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAABGdBTUEAALGPC/xhBQAAAAZiS0dEAP8A/wD/oL2nkwAAAAlwSFlzAAAlywAAJcsBGkdkZgAAAAd0SU1FB+MDHQ02AU8UBK4AAASBSURBVFjDtZfNb1RlFMZ/7zt3vqCU0pSSQjMSbCHVRSEjq7aBhQvFsDBqJIYF/gGGxAUpREyMCG5YqCSuiDGGRE2NCxTRBgkJuGlKYpAOBW3NtJ22UzoznTv3du6d++FibsvQ6XzQ1rO899z3Oefc5zznvII6LRahCTgCHAIOAu3ANu91GpgEhoBbwLWuOJl6zhV1AO8F+oG3gU11xqsD3wMXuuI8XFMAsQhh4BzwHuBnbVYAvgDOdsXR6w4gFqET+BF4kY2x+8DrXXEe1QwgFuEA8CuwnY21Od3h1egkwxUD8DK/8z+AozugOaQl9PQmiJUF4P3zoQ0s+0rwJRv1QbQngQaglPidqwYuWjtp+eRzELIqmF0wyefzy7kVcira6XdLXfY58KlH7qKX12p/VWO7cvA1OgZ+wrVthM+HY1lIRXnKx7EsMpkMOVXFJ8AwDMJ+P3/3daw8zpLQ3ZtgZCmd0/W22r/jY6jpFLdv/o6xuFgGbrsuqqqyuJDhcTJZMR8XzgAIT+GmaonMcgUsC6H4sAsWPr+/DBxgMaciERhGnk2BwGoVAMj7BO2KJ681Fc6ZHCd18waIculYXFjAdoosU/buI9y6Ayu7gDUyQjqnVjoy5LgcVYDD9ZTemR4heeLlmmxvvHydzaEw8Y/OYt74ttaxhxTgpdIn4eOnCHR0epS2KmusrmOXyEh28Be0u4OEZmeYv/J1PeC4EFW8qbZszW8dY8v+A1U/zGsaqpYjr+lIn8QfDKEWTLg7yHz/ibr1QUBEKRmpT/6pmiWZTBIIhjBNk+f27CkBz6FqOq5tMzUxQbChgdYdO9YkUC40KquyY3MDO3cFED6J67hl4AiBkJKtzdtQAkGklGtWSQVIAa3L2T8aRQYDBNt2oTQ14Zgm2v17WKaJlsvhel1guRD2vjFmZzDj8WcGF5AVsQh3gbKfrvS+SeSzLxE+H//sb66m7euxe4o3gMoCsG4PED8JbR9+7EUUJHzsJKaU+FxorDgMbArGk1ng6BrG1cuVKjAsYhHeAa5ULFOgAdfMoRw8QsfAzzVY5ZJNp9GNPAKBbdtI0+RR3/OVAjghgWveDrf6mWauSEAvo+nJCbRslj+HhjAN42nwTBrDtpmfe4yanGFyfKxqN0vBVeltr9/VUjjda4amrU2EwmEiu3fj92ZBETyDYdkABPx+lGCIcDhcjYA/9EyRWmrDC8Dx1SbiEuH8mRT67AwCganrhID83BwA2kKGfKFQpMqWRppaWnAdG7/jYCRnV8O3BJxfuRFdBN5fL9vbvrpOY3c301e+IXvxVKXsL/UligtJqYJ84G2v62o117KqggOjUtBfaSntAP7QHbavtc/lnijO2HCl1ylvKX1QcS0fbieqOfwGNG/wbpqS8EpvgqGnAl7pFZ1kWEIPMLqB4KNe5kNlFVvNuzfBA0UQFcVrlbUOYEvAJQWipWV/psvp7Z284MIZF94AQnUCGwIGBJzvTTCyrtvxkt3ZRbPjchQ47EJUQGRpJAjIuhAXMAzckoKrPVPM13Puf2Qd6X5KXrjXAAAAAElFTkSuQmCC",
			MediaType: "image/png",
		},
	}
	tLabels := map[string]string{
		"alm-owner-" + csv.Name: operatorName,
		"operated-by":           csvVersionedName,
	}
	templateStruct.Spec.Labels = tLabels
	templateStruct.Spec.Selector = &metav1.LabelSelector{MatchLabels: tLabels}
	templateStruct.Spec.InstallModes = []csvv1.InstallMode{
		{Type: csvv1.InstallModeTypeOwnNamespace, Supported: true},
		{Type: csvv1.InstallModeTypeSingleNamespace, Supported: false},
		{Type: csvv1.InstallModeTypeMultiNamespace, Supported: false},
		{Type: csvv1.InstallModeTypeAllNamespaces, Supported: false},
	}
	templateStruct.Spec.CustomResourceDefinitions.Owned = []csvv1.CRDDescription{
		{
			Version:     api.SchemeGroupVersion.Version,
			Kind:        "ActiveMQArtemis",
			DisplayName: "ActiveMQ Artemis",
			Description: "An instance of Active MQ Artemis.",
			Name:        "activemqartemises." + api.SchemeGroupVersion.Group,
			Resources: []csvv1.APIResourceReference{

				{
					Kind:    "StatefulSet",
					Version: appsv1.SchemeGroupVersion.String(),
				},
				{
					Kind:    "Secret",
					Version: corev1.SchemeGroupVersion.String(),
				},
				{
					Kind:    "Service",
					Version: corev1.SchemeGroupVersion.String(),
				},

				{
					Kind:    "ImageStream",
					Version: oimagev1.SchemeGroupVersion.String(),
				},
			},
			SpecDescriptors: []csvv1.SpecDescriptor{

				{
					Description:  "User name for standard broker user.It is required for connecting to the broker and the web console.If left empty, it will be generated.",
					DisplayName:  "AdminUser",
					Path:         "adminUser",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:AdminUser", "urn:alm:descriptor:com.tectonic.ui:text"},
				},

				{
					Description:  "Password for standard broker user.It is required for connecting to the broker and the web console.If left empty, it will be generated.",
					DisplayName:  "AdminPassword",
					Path:         "adminPassword",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:AdminUser", "urn:alm:descriptor:com.tectonic.ui:password"},
				},
				{
					Description:  "The version of the broker deployment.",
					DisplayName:  "Version",
					Path:         "version",
					XDescriptors: []string{"uurn:alm:descriptor:com.tectonic.ui:fieldGroup:AdminUser", "urn:alm:descriptor:com.tectonic.ui:label"},
				},
				{
					Description:  "The number of broker pods to deploy",
					DisplayName:  "Size",
					Path:         "deploymentPlan.size",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:deploymentPlan", "urn:alm:descriptor:com.tectonic.ui:podCount"},
				},
				{
					Description:  "The image used for the broker deployment",
					DisplayName:  "Image",
					Path:         "deploymentPlan.image",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:deploymentPlan", "urn:alm:descriptor:com.tectonic.ui:text"},
				},
				{
					Description: "If true require user password login credentials for broker protocol ports",
					DisplayName: "RequireLogin",
					Path:        "deploymentPlan.requireLogin",
					XDescriptors: []string{
						"urn:alm:descriptor:com.tectonic.ui:fieldGroup:deploymentPlan", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				},
				{
					Description: "If true use persistent volume via persistent volume claim for journal storage",

					DisplayName:  "PersistenceEnabled",
					Path:         "deploymentPlan.persistenceEnabled",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:deploymentPlan", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				},

				{
					Description:  "If aio use ASYNCIO, if nio use NIO for journal IO",
					DisplayName:  "JournalType",
					Path:         "deploymentPlan.journalType",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:deploymentPlan", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "If true migrate messages on scaledown",
					DisplayName:  "MessageMigration",
					Path:         "deploymentPlan.messageMigration",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:deploymentPlan", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {

					Description:  "A single acceptor configuration",
					DisplayName:  "Name",
					Path:         "acceptors[0].name",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Port number",
					DisplayName:  "Port",
					Path:         "acceptors[0].port",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:number"},
				}, {
					Description:  "The protocols to enable for this acceptor",
					DisplayName:  "Protocols",
					Path:         "acceptors[0].protocols",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Whether or not to enable SSL on this port",
					DisplayName:  "Ssl Enabled",
					Path:         "acceptors[0].sslEnabled",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Name of the secret to use for ssl information",
					DisplayName:  "Ssl Secret",
					Path:         "acceptors[0].sslSecret",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Comma separated list of cipher suites used for SSL communication.",
					DisplayName:  "Enabled Cipher Suites",
					Path:         "acceptors[0].enabledCipherSuites",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Comma separated list of protocols used for SSL communication.",
					DisplayName:  "Enabled Protocols",
					Path:         "acceptors[0].enabledProtocols",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Tells a client connecting to this acceptor that 2-way SSL is required.This property takes precedence over wantClientAuth.",
					DisplayName:  "Need Client Auth",
					Path:         "acceptors[0].needClientAuth",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Tells a client connecting to this acceptor that 2-way SSL is requested but not required.Overridden by needClientAuth.",
					DisplayName:  "Want Client Auth",
					Path:         "acceptors[0].wantClientAuth",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "The CN of the connecting client's SSL certificate will be compared to its hostname to verify they match.This is useful only for 2-way SSL.",
					DisplayName:  " Verify Host",
					Path:         "acceptors[0].verifyHost",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Used to change the SSL Provider between JDK and OPENSSL.The default is JDK.",
					DisplayName:  "Ssl Provider",
					Path:         "acceptors[0].sslProvider",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "A regular expression used to match the server_name extension on incoming SSL connections.If the name doesn't match then the connection to the acceptor will be rejected.",
					DisplayName:  "Sni Host",
					Path:         "acceptors[0].sniHost",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Whether or not to expose this acceptor",
					DisplayName:  "Expose",
					Path:         "acceptors[0].expose",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  " To indicate which kind of routing type to use.",
					DisplayName:  "AnyCast Prefix",
					Path:         "acceptors[0].anycastPrefix",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "To indicate which kind of routing type to use",
					DisplayName:  "Multicast Prefix",
					Path:         "acceptors[0].multicastPrefix",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "To indicate which kind of routing type to use",
					DisplayName:  "Connections Allowed",
					Path:         "acceptors[0].connectionsAllowed",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:acceptors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {

					Description:  "The name of the connector",
					DisplayName:  "Name",
					Path:         "connectors[0].name",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "The type either tcp or vm",
					DisplayName:  "Type",
					Path:         "connectors[0].type",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Hostname or IP to connect to",
					DisplayName:  "Host Name",
					Path:         "connectors[0].host",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Port number",
					DisplayName:  "Port",
					Path:         "connectors[0].port",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:number"},
				}, {
					Description:  "Whether or not to enable SSL on this port",
					DisplayName:  "Ssl Enabled",
					Path:         "connectors[0].sslEnabled",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Name of the secret to use for ssl information",
					DisplayName:  "Ssl Secret",
					Path:         "connectors[0].sslSecret",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Comma separated list of cipher suites used for SSL communication.",
					DisplayName:  "Enabled Cipher Suites",
					Path:         "connectors[0].enabledCipherSuites",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Comma separated list of protocols used for SSL communication.",
					DisplayName:  "Enabled Protocols",
					Path:         "connectors[0].enabledProtocols",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Tells a client connecting to this connector that 2-way SSL is required.This property takes precedence over wantClientAuth.",
					DisplayName:  "Need Client Auth",
					Path:         "connectors[0].needClientAuth",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Tells a client connecting to this connector that 2-way SSL is requested but not required.Overridden by needClientAuth.",
					DisplayName:  "Want Client Auth",
					Path:         "connectors[0].wantClientAuth",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "The CN of the connecting client's SSL certificate will be compared to its hostname to verify they match.This is useful only for 2-way SSL.",
					DisplayName:  "Verify Host",
					Path:         "connectors[0].verifyHost",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Used to change the SSL Provider between JDK and OPENSSL.The default is JDK.",
					DisplayName:  "Ssl Provider",
					Path:         "connectors[0].sslProvider",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "A regular expression used to match the server_name extension on incoming SSL connections.If the name doesn't match then the connection to the acceptor will be rejected.",
					DisplayName:  "Sni Host",
					Path:         "connectors[0].sniHost",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "Whether or not to expose this connector",
					DisplayName:  "Expose",
					Path:         "connectors[0].expose",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:connectors", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Whether or not to expose this port",
					DisplayName:  "Expose",
					Path:         "console.expose",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:console", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Whether or not to enable SSL on this port",
					DisplayName:  "Ssl Enabled",
					Path:         "console.sslEnabled",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:console", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				}, {
					Description:  "Name of the secret to use for ssl information",
					DisplayName:  "Ssl Secret",
					Path:         "console.sslSecret",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:console", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "If the embedded server requires client authentication",
					DisplayName:  "Use Client Auth",
					Path:         "console.useClientAuth",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:console", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				},
				{
					Description:  "Set true to enable automatic micro version product upgrades, it is disabled by default.",
					DisplayName:  "Enable Upgrades",
					Path:         "upgrades.enabled",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:upgrades", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				},
				{
					Description:  "Set true to enable automatic minor product version upgrades, it is disabled by default. Requires spec.upgrades.enabled to be true.",
					DisplayName:  "Include minor version upgrades",
					Path:         "upgrades.minor",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:upgrades", " urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				},
			},
			StatusDescriptors: []csvv1.StatusDescriptor{
				{
					Description:  "Deployments for the  Activemq artemises environment.",
					DisplayName:  "Pods Status",
					Path:         "podStatus",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:podStatuses"},
				},
			},
		}, {
			Version:     v2alpha1.SchemeGroupVersion.Version,
			Kind:        "ActiveMQArtemisAddress",
			DisplayName: "ActiveMQ Artemis Address",
			Description: "Adding and removing addresses via custom resource definitions.",
			Name:        "activemqartemisaddresses." + v2alpha1.SchemeGroupVersion.Group,
			SpecDescriptors: []csvv1.SpecDescriptor{

				{
					Description:  "The Queue Name",
					DisplayName:  "Queue Name",
					Path:         "queueName",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:ActiveMQArtemis", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "The Address Name",
					DisplayName:  "Address Name",
					Path:         "addressName",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:ActiveMQArtemis", "urn:alm:descriptor:com.tectonic.ui:text"},
				}, {
					Description:  "The Routing Type",
					DisplayName:  "Routing Type",
					Path:         "routingType",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:ActiveMQArtemis", "urn:alm:descriptor:com.tectonic.ui:text"},
				},
			},
		}, {
			Version:     v2alpha1.SchemeGroupVersion.Version,
			Kind:        "ActiveMQArtemisScaledown",
			DisplayName: "ActiveMQ Artemis Scaledown",
			Description: "Provides message migration on clustered broker scaledown.",
			Name:        "activemqartemisscaledowns." + v2alpha1.SchemeGroupVersion.Group,
			SpecDescriptors: []csvv1.SpecDescriptor{

				{Description: "Triggered by main ActiveMQArtemis CRD messageMigration entry",
					DisplayName:  "LocalOnly",
					Path:         "localOnly",
					XDescriptors: []string{"urn:alm:descriptor:com.tectonic.ui:fieldGroup:SingleNamespace", "urn:alm:descriptor:com.tectonic.ui:booleanSwitch"},
				},
			},
		},
	}

	opMajor, opMinor, opMicro := activemqartemis.MajorMinorMicro(version.Version)
	csvFile := "deploy/olm-catalog/" + csv.CsvDir + "/" + opMajor + "." + opMinor + "." + opMicro + "/" + csvVersionedName + ".clusterserviceversion.yaml"

	imageName, _, _ := activemqartemis.GetImage(deployment.Spec.Template.Spec.Containers[0].Image)
	relatedImages := []image{}

	if csv.OperatorName == "activemq-artemis-operator" {
		templateStruct.Annotations["certified"] = "false"
		deployFile := "deploy/operator.yaml"
		createFile(deployFile, deployment)
		roleFile := "deploy/role.yaml"
		createFile(roleFile, role)
	}
	relatedImages = append(relatedImages, image{Name: imageName, Image: deployment.Spec.Template.Spec.Containers[0].Image})

	imageRef := constants.ImageRef{
		TypeMeta: metav1.TypeMeta{
			APIVersion: oimagev1.SchemeGroupVersion.String(),
			Kind:       "ImageStream",
		},
		Spec: constants.ImageRefSpec{
			Tags: []constants.ImageRefTag{
				{
					// Needs to match the component name for upstream and downstream.
					Name: "amq7/amq-broker-rhel8-operator",
					From: &corev1.ObjectReference{
						// Needs to match the image that is in your CSV that you want to replace.
						Name: deployment.Spec.Template.Spec.Containers[0].Image,
						Kind: "DockerImage",
					},
				},
			},
		},
	}

	sort.Sort(sort.Reverse(sort.StringSlice(activemqartemis.SupportedVersions)))

	imageRef.Spec.Tags = append(imageRef.Spec.Tags, constants.ImageRefTag{
		Name: constants.Broker75Component,
		From: &corev1.ObjectReference{
			Name: constants.Broker75ImageURL,
			Kind: "DockerImage",
		},
	})

	relatedImages = append(relatedImages, getRelatedImage(constants.Broker75ImageURL))

	if GetBoolEnv("DIGESTS") {

		for _, tagRef := range imageRef.Spec.Tags {

			if _, ok := imageShaMap[tagRef.From.Name]; !ok {
				imageShaMap[tagRef.From.Name] = ""
				imageName, imageTag, imageContext := activemqartemis.GetImage(tagRef.From.Name)
				repo := imageContext + "/" + imageName

				if strings.Contains(tagRef.From.Name, "quay.io") {
					digests, err := util.RetriveFromQuayIO(repo, imageTag)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
					if len(digests) > 0 {
						imageShaMap[tagRef.From.Name] = strings.ReplaceAll(tagRef.From.Name, ":"+imageTag, "@"+digests[0])
					} else {
						fmt.Printf("could not get the digest for tag %s from registry %s \n", imageTag, tagRef.From.Name)
					}

				} else {

					digests, err := util.RetriveFromRedHatIO(repo, imageTag)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
					if len(digests) > 1 {
						imageShaMap[tagRef.From.Name] = strings.ReplaceAll(tagRef.From.Name, ":"+imageTag, "@"+digests[len(digests)-1])
					}

				}
			}
		}
	}

	//not sure if we required mage-references file in the future So comment out for now.

	//imageFile := "deploy/olm-catalog/" + csv.CsvDir + "/" + opMajor + "." + opMinor + "." + opMicro + "/" + "image-references"
	//createFile(imageFile, imageRef)

	var templateInterface interface{}
	if len(relatedImages) > 0 {
		templateJSON, err := json.Marshal(templateStruct)
		if err != nil {
			fmt.Println(err)
		}
		result, err := sjson.SetBytes(templateJSON, "spec.relatedImages", relatedImages)
		if err != nil {
			fmt.Println(err)

		}
		if err = json.Unmarshal(result, &templateInterface); err != nil {
			fmt.Println(err)
		}
	} else {
		templateInterface = templateStruct
	}

	// find and replace images with SHAs where necessary
	templateByte, err := json.Marshal(templateInterface)
	if err != nil {
		fmt.Println(err)
	}
	for from, to := range imageShaMap {
		if to != "" {
			templateByte = bytes.ReplaceAll(templateByte, []byte(from), []byte(to))
		}
	}
	if err = json.Unmarshal(templateByte, &templateInterface); err != nil {
		fmt.Println(err)
	}
	createFile(csvFile, &templateInterface)

	packageFile := "deploy/olm-catalog/" + csv.CsvDir + "/" + csv.Name + ".package.yaml"
	p, err := os.Create(packageFile)
	defer p.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	pwr := bufio.NewWriter(p)
	pwr.WriteString("#! package-manifest: " + csvFile + "\n")
	packagedata := packageStruct{
		PackageName: operatorName,
		Channels: []channel{
			{
				Name:       "current-76",
				CurrentCSV: operatorName + ".v" + version.PriorVersion,
			},
			{
				Name:       maturity,
				CurrentCSV: csvVersionedName,
			},
		},
		DefaultChannel: maturity,
	}
	util.MarshallObject(packagedata, pwr)
	pwr.Flush()
}

type csvSetting struct {
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	OperatorName string `json:"operatorName"`
	CsvDir       string `json:"csvDir"`
	Registry     string `json:"repository"`
	Context      string `json:"context"`
	ImageName    string `json:"imageName"`
	Tag          string `json:"tag"`
}
type csvPermissions struct {
	ServiceAccountName string              `json:"serviceAccountName"`
	Rules              []rbacv1.PolicyRule `json:"rules"`
}
type csvDeployments struct {
	Name string                `json:"name"`
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}
type csvStrategySpec struct {
	Permissions        []csvPermissions                `json:"permissions"`
	Deployments        []csvDeployments                `json:"deployments"`
	ClusterPermissions []StrategyDeploymentPermissions `json:"clusterPermissions,omitempty"`
}

type StrategyDeploymentPermissions struct {
	ServiceAccountName string            `json:"serviceAccountName"`
	Rules              []rbac.PolicyRule `json:"rules"`
}
type channel struct {
	Name       string `json:"name"`
	CurrentCSV string `json:"currentCSV"`
}
type packageStruct struct {
	PackageName    string    `json:"packageName"`
	Channels       []channel `json:"channels"`
	DefaultChannel string    `json:"defaultChannel"`
}
type image struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

func getRelatedImage(imageURL string) image {
	imageName, _, _ := activemqartemis.GetImage(imageURL)
	return image{
		Name:  imageName,
		Image: imageURL,
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func createFile(filepath string, obj interface{}) {
	f, err := os.Create(filepath)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	writer := bufio.NewWriter(f)
	util.MarshallObject(obj, writer)
	writer.Flush()
}

func GetBoolEnv(key string) bool {
	val := GetEnv(key, "false")
	ret, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return ret
}

func GetEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}
