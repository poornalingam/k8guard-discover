package discover

import (
	lib "github.com/k8guard/k8guardlibs"
	"k8s.io/client-go/pkg/api/v1"
	"github.com/k8guard/k8guard-discover/metrics"
	"github.com/k8guard/k8guardlibs/messaging/kafka"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/k8guard/k8guardlibs/violations"
	"strings"
	"encoding/json"
)

func isIgnoredNamespace(namespace string) bool {
	for _, n := range lib.Cfg.IgnoredNamespaces {
		if n == namespace {
			return true
		}
	}
	return false
}

func GetAllNamspacesFromApi() []v1.Namespace {
	namespaces := Clientset.Namespaces()

	namespaceList, err := namespaces.List(v1.ListOptions{})

	if err != nil {
		lib.Log.Error("error: ", err)
		panic(err.Error())
	}

	metrics.Update(metrics.ALL_NAMESPACE_COUNT, len(namespaceList.Items))

	return namespaceList.Items
}

func GetBadNamespaces(theNamespaces []v1.Namespace, sendToKafka bool) []lib.Namespace {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(metrics.FNGetBadNamespaces.Set))
	defer timer.ObserveDuration()

	allBadNamespaces := []lib.Namespace{}

	for _, kn := range theNamespaces {
		if isIgnoredNamespace(kn.Namespace) == true {
			continue
		}
		n := lib.Namespace{}
		n.Name = kn.Name
		n.Namespace = kn.Name
		n.Cluster = lib.Cfg.ClusterName
		// this one feels weird but to be consistent

		if hasOwnerAnnotation(kn,lib.Cfg.AnnotationFormatForEmails) == false && hasOwnerAnnotation(kn,lib.Cfg.AnnotationFormatForChatIds) == false && isNotIgnoredViloation(violations.NO_OWNER_ANNOTATION_TYPE) {
			jsonString, err := json.Marshal(kn.Annotations)
			if err != nil {
				lib.Log.Error("Can not convert annotation to a valid json ",err)

			}
			n.Violations = append(n.Violations, violations.Violation{Source: string(jsonString), Type: violations.NO_OWNER_ANNOTATION_TYPE})
		}

		if len(n.ViolatableEntity.Violations) > 0 {
			allBadNamespaces = append(allBadNamespaces, n)
			if sendToKafka {
				lib.Log.Debug("Sending ", n.Name, " to kafka")
				err := KafkaProducer.SendData(lib.Cfg.KafkaActionTopic, kafka.NAMESPACE_MESSAGE, n)
				if err != nil {
					panic(err)
				}
			}
		}

	}
	metrics.Update(metrics.BAD_NAMESPACE_COUNT, len(allBadNamespaces))
	return allBadNamespaces
}

func hasOwnerAnnotation(namespace v1.Namespace,annotationKind string) bool {
	teamString, ok := namespace.Annotations[annotationKind];
	if ok {
		team := strings.Split(teamString, ",")
		if len(team) > 0 {
			return true
		}
	}
	return false
}
