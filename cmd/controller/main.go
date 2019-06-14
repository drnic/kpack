package main

import (
	"flag"
	"log"
	"sync"
	"time"

	knversioned "github.com/knative/build/pkg/client/clientset/versioned"
	knexternalversions "github.com/knative/build/pkg/client/informers/externalversions"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbbuild"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbbuilder"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbimage"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	routinesPerController = 2
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
)

func main() {
	flag.Parse()
	devLogger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Couldn't create logger: %s", err)
	}
	logger := devLogger.Sugar()

	clusterConfig, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	cnbClient, err := versioned.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get CNBBuild client: %s", err.Error())
	}

	knbuildClient, err := knversioned.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get Knative Build client: %s", err.Error())
	}

	k8sClient, err := v1.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err.Error())
	}

	options := reconciler.Options{
		Logger:       logger,
		CNBClient:    cnbClient,
		ResyncPeriod: 10 * time.Hour,
	}

	cnbInformerFactory := externalversions.NewSharedInformerFactory(cnbClient, options.ResyncPeriod)
	cnbBuildInformer := cnbInformerFactory.Build().V1alpha1().CNBBuilds()
	cnbImageInformer := cnbInformerFactory.Build().V1alpha1().CNBImages()
	cnbBuilderInformer := cnbInformerFactory.Build().V1alpha1().CNBBuilders()

	knBuildInformerFactory := knexternalversions.NewSharedInformerFactory(knbuildClient, options.ResyncPeriod)
	knBuildInformer := knBuildInformerFactory.Build().V1alpha1().Builds()

	metadataRetriever := &registry.RemoteMetadataRetriever{
		LifecycleImageFactory: &registry.ImageFactory{
			KeychainFactory: &registry.SecretKeychainFactory{
				SecretManager: &registry.SecretManager{
					Client: k8sClient,
				},
			},
		},
	}

	buildController := cnbbuild.NewController(options, knbuildClient, cnbBuildInformer, knBuildInformer, metadataRetriever)
	imageController := cnbimage.NewController(options, cnbImageInformer, cnbBuildInformer, cnbBuilderInformer)
	builderController := cnbbuilder.NewController(options, cnbBuilderInformer, metadataRetriever)

	stopChan := make(chan struct{})
	cnbInformerFactory.Start(stopChan)
	knBuildInformerFactory.Start(stopChan)

	cache.WaitForCacheSync(stopChan, cnbBuildInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, cnbImageInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, cnbBuilderInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, knBuildInformer.Informer().HasSynced)

	err = runGroup(
		func(done <-chan struct{}) error {
			return imageController.Run(routinesPerController, done)
		},
		func(done <-chan struct{}) error {
			return buildController.Run(routinesPerController, done)
		},
		func(done <-chan struct{}) error {
			return builderController.Run(routinesPerController, done)
		},
	)
	if err != nil {
		logger.Fatalw("Error running controller", zap.Error(err))
	}
}

type doneFunc func(done <-chan struct{}) error

func runGroup(fns ...doneFunc) error {
	var wg sync.WaitGroup
	wg.Add(len(fns))

	done := make(chan struct{})
	result := make(chan error, len(fns))
	for _, fn := range fns {
		go func(fn doneFunc) {
			defer wg.Done()
			result <- fn(done)
		}(fn)
	}

	defer close(result)
	defer wg.Wait()
	defer close(done)
	return <-result
}
