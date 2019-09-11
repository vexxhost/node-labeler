package main

import (
	"encoding/json"
	"os"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/gosimple/slug"
	"github.com/jaypipes/ghw"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var log *zap.Logger

func main() {
	// NOTE(mnaser): Due to the fact that we don't want to run ghw in
	//               a privileged context, we simply disable all warnings.
	os.Setenv("GHW_DISABLE_WARNINGS", "1")

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	log = logger.Named("node-labeler")

	nodeName, ok := os.LookupEnv("NODE")
	if !ok {
		log.Fatal("Environment variable is not defined",
			zap.String("var", "NODE"),
		)
	}

	log.Info("Starting service",
		zap.String("node", nodeName),
	)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	for {
		dmi, err := ghw.DMI()
		if err != nil {
			log.Fatal(err.Error())
		}

		labels := map[string]string{
			"node.vexxhost.com/vendor":  slug.Make(dmi.System.Vendor),
			"node.vexxhost.com/product": slug.Make(dmi.Product.Name),
		}

		node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			log.Fatal(err.Error())
		}

		for label, value := range labels {
			addLabelToNode(clientset, node, label, value)
		}

		time.Sleep(600 * time.Second)
	}

}

func addLabelToNode(clientset *kubernetes.Clientset, node *v1.Node, key string, value string) error {
	log.Info("Applying node label",
		zap.String(key, value),
	)

	originalNode, err := json.Marshal(node)
	if err != nil {
		log.Fatal(err.Error())
	}

	node.ObjectMeta.Labels[key] = value

	newNode, err := json.Marshal(node)
	if err != nil {
		log.Fatal(err.Error())
	}

	patch, err := jsonpatch.CreateMergePatch(originalNode, newNode)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Info("Patching Node resource",
		zap.String("node", node.ObjectMeta.Name),
		zap.String("patch", string(patch)),
	)

	_, err = clientset.CoreV1().Nodes().Patch(node.Name, types.MergePatchType, patch)
	if err != nil {
		log.Error(err.Error())
	}

	return err
}
