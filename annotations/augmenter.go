package annotations

import (
	"context"
	"strings"

	"github.com/Financial-Times/draft-annotations-api/concept"
	"github.com/Financial-Times/draft-annotations-api/mapper"
	tidUtils "github.com/Financial-Times/transactionid-utils-go"
	log "github.com/sirupsen/logrus"
)

type Augmenter struct {
	conceptRead concept.ReadAPI
}

func NewAugmenter(api concept.ReadAPI) *Augmenter {
	return &Augmenter{api}
}

func (a *Augmenter) AugmentAnnotations(ctx context.Context, canonicalAnnotations []Annotation) ([]Annotation, error) {
	tid, err := tidUtils.GetTransactionIDFromContext(ctx)

	if err != nil {
		tid = tidUtils.NewTransactionID()
		log.WithField(tidUtils.TransactionIDKey, tid).
			WithError(err).
			Warn("Transaction ID error in augmenting annotations with concept data: Generated a new transaction ID")
		ctx = tidUtils.TransactionAwareContext(ctx, tid)
	}

	dedupedCanonical := dedupeCanonicalAnnotations(canonicalAnnotations)
	dedupedCanonical = filterOutInvalidPredicates(dedupedCanonical)

	uuids := getConceptUUIDs(dedupedCanonical)

	concepts, err := a.conceptRead.GetConceptsByIDs(ctx, uuids)

	if err != nil {
		log.WithField(tidUtils.TransactionIDKey, tid).
			WithError(err).Error("Request failed when attempting to augment annotations from UPP concept data")
		return nil, err
	}

	augmentedAnnotations := make([]Annotation, 0)
	for _, ann := range dedupedCanonical {
		uuid := extractUUID(ann.ConceptId)
		concept, found := concepts[uuid]
		if found {
			ann.ConceptId = concept.ID
			ann.ApiUrl = concept.ApiUrl
			ann.PrefLabel = concept.PrefLabel
			ann.IsFTAuthor = concept.IsFTAuthor
			ann.Type = concept.Type
			augmentedAnnotations = append(augmentedAnnotations, ann)
		} else {
			log.WithField(tidUtils.TransactionIDKey, tid).
				WithField("conceptId", ann.ConceptId).
				Warn("Concept data for this annotation was not found, and will be removed from the list of annotations.")
		}
	}

	log.WithField(tidUtils.TransactionIDKey, tid).Info("Annotations augmented with concept data")
	return augmentedAnnotations, nil
}

func dedupeCanonicalAnnotations(annotations []Annotation) []Annotation {
	var empty struct{}
	var deduped []Annotation
	dedupedMap := make(map[Annotation]struct{})
	for _, ann := range annotations {
		dedupedMap[ann] = empty
	}
	for k := range dedupedMap {
		deduped = append(deduped, k)
	}
	return deduped
}

func filterOutInvalidPredicates(annotations []Annotation) []Annotation {
	i := 0
	for _, item := range annotations {
		if !mapper.IsValidPACPredicate(item.Predicate) {
			continue
		}
		annotations[i] = item
		i++
	}

	return annotations[:i]
}

func getConceptUUIDs(canonicalAnnotations []Annotation) []string {
	conceptUUIDs := make(map[string]struct{})
	var empty struct{}
	var keys []string
	for _, ann := range canonicalAnnotations {
		conceptUUID := extractUUID(ann.ConceptId)
		if conceptUUID != "" {
			conceptUUIDs[conceptUUID] = empty
		}
	}
	for k := range conceptUUIDs {
		keys = append(keys, k)

	}
	return keys
}

func extractUUID(conceptID string) string {
	i := strings.LastIndex(conceptID, "/")
	if i == -1 || i == len(conceptID)-1 {
		return ""
	}
	return conceptID[i+1:]
}
