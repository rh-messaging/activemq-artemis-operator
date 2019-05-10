package env

import (
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"k8s.io/client-go/rest"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	v1 "k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

var log = logf.Log.WithName("env")

func DetectOpenshift() (bool, error) {

	log.Info("Detect if openshift is running")

	value, ok := os.LookupEnv("OPERATOR_OPENSHIFT")
	if ok {
		log.Info("Set by env-var 'OPERATOR_OPENSHIFT': " + value)
		return strings.ToLower(value) == "true", nil
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Error getting config: %v")
		return false, err
	}

	groupName := "route.openshift.io"
	gv := v1.GroupVersion{Group: groupName, Version: "v1"}
	cfg.APIPath = "/apis"

	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	cfg.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: codecs}

	if cfg.UserAgent == "" {
		cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	cfg.GroupVersion = &gv

	client, err := rest.RESTClientFor(cfg)

	if err != nil {
		log.Error(err, "Error getting client: %v")
		return false, err
	}

	_, err = client.Get().DoRaw()

	return err == nil, nil
}

