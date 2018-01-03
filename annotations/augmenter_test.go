package annotations

import (
	"context"
	"errors"
	"testing"

	"github.com/Financial-Times/draft-annotations-api/concept"
	tidUtils "github.com/Financial-Times/transactionid-utils-go"
	log "github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var testCanonicalizedAnnotations = []Annotation{
	{
		Predicate: "http://www.ft.com/ontology/classification/isClassifiedBy",
		ConceptId: "http://www.ft.com/thing/b224ad07-c818-3ad6-94af-a4d351dbb619",
	},
	{
		Predicate: "http://www.ft.com/ontology/annotation/mentions",
		ConceptId: "http://www.ft.com/thing/1a2a1a0a-7199-38b8-8a73-e651e2172471",
	},
	{
		Predicate: "http://www.ft.com/ontology/hasContributor",
		ConceptId: "http://www.ft.com/thing/5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b",
	},
	{
		Predicate: "http://www.ft.com/ontology/annotation/mentions",
		ConceptId: "http://www.ft.com/thing/1fb3faf1-bf00-3a15-8efb-1038a59653f7",
	},
	{
		Predicate: "http://www.ft.com/ontology/annotation/mentions",
		ConceptId: "http://www.ft.com/thing/7b7dafa0-d54e-4c1d-8e22-3d452792acd2",
	},
}

var testConceptIDs = []string{
	"b224ad07-c818-3ad6-94af-a4d351dbb619",
	"1a2a1a0a-7199-38b8-8a73-e651e2172471",
	"5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b",
	"1fb3faf1-bf00-3a15-8efb-1038a59653f7",
	"7b7dafa0-d54e-4c1d-8e22-3d452792acd2",
}

var testConcepts = map[string]concept.Concept{
	"b224ad07-c818-3ad6-94af-a4d351dbb619": {
		ID:        "http://www.ft.com/thing/b224ad07-c818-3ad6-94af-a4d351dbb619",
		ApiUrl:    "http://api.ft.com/things/b224ad07-c818-3ad6-94af-a4d351dbb619",
		Type:      "http://www.ft.com/ontology/Subject",
		PrefLabel: "Economic Indicators",
	},
	"5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b": {
		ID:         "http://www.ft.com/thing/5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b",
		ApiUrl:     "http://api.ft.com/things/5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b",
		Type:       "http://www.ft.com/ontology/person/Person",
		PrefLabel:  "Lisa Barrett",
		IsFTAuthor: true,
	},
	"7b7dafa0-d54e-4c1d-8e22-3d452792acd2": {
		ID:         "http://www.ft.com/thing/28f8d585-37ea-4879-ae1c-f6c0580a43b8",
		ApiUrl:     "http://api.ft.com/things/28f8d585-37ea-4879-ae1c-f6c0580a43b8",
		Type:       "http://www.ft.com/ontology/person/Person",
		PrefLabel:  "Frederick Stapleton",
		IsFTAuthor: false,
	},
}

var expectedAugmentedAnnotations = []Annotation{
	{
		Predicate:  "http://www.ft.com/ontology/classification/isClassifiedBy",
		ConceptId:  "http://www.ft.com/thing/b224ad07-c818-3ad6-94af-a4d351dbb619",
		ApiUrl:     "http://api.ft.com/things/b224ad07-c818-3ad6-94af-a4d351dbb619",
		Type:       "http://www.ft.com/ontology/Subject",
		PrefLabel:  "Economic Indicators",
		IsFTAuthor: false,
	},
	{
		Predicate:  "http://www.ft.com/ontology/hasContributor",
		ConceptId:  "http://www.ft.com/thing/5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b",
		ApiUrl:     "http://api.ft.com/things/5bd49568-6d7c-3c10-a5b0-2f3fd5974a6b",
		Type:       "http://www.ft.com/ontology/person/Person",
		PrefLabel:  "Lisa Barrett",
		IsFTAuthor: true,
	},
	{
		Predicate:  "http://www.ft.com/ontology/annotation/mentions",
		ConceptId:  "http://www.ft.com/thing/28f8d585-37ea-4879-ae1c-f6c0580a43b8",
		ApiUrl:     "http://api.ft.com/things/28f8d585-37ea-4879-ae1c-f6c0580a43b8",
		Type:       "http://www.ft.com/ontology/person/Person",
		PrefLabel:  "Frederick Stapleton",
		IsFTAuthor: false,
	},
}

func TestAugmentAnnotations(t *testing.T) {
	conceptsSearchAPI := new(ConceptSearchAPIMock)
	ctx := tidUtils.TransactionAwareContext(context.Background(), tidUtils.NewTransactionID())
	conceptsSearchAPI.
		On("SearchConcepts", ctx, testConceptIDs).
		Return(testConcepts, nil)
	a := NewAugmenter(conceptsSearchAPI)

	annotations, err := a.AugmentAnnotations(ctx, testCanonicalizedAnnotations)

	assert.NoError(t, err)
	assert.Equal(t, len(expectedAugmentedAnnotations), len(annotations))
	for _, expected := range expectedAugmentedAnnotations {
		assert.Contains(t, annotations, expected)
	}

	conceptsSearchAPI.AssertExpectations(t)
}

func TestAugmentAnnotationsMissingTransactionID(t *testing.T) {
	hook := logTest.NewGlobal()
	conceptsSearchAPI := new(ConceptSearchAPIMock)
	conceptsSearchAPI.
		On("SearchConcepts", mock.Anything, testConceptIDs).
		Return(testConcepts, nil)
	a := NewAugmenter(conceptsSearchAPI)

	a.AugmentAnnotations(context.Background(), testCanonicalizedAnnotations)

	var tid string
	for i, e := range hook.AllEntries() {
		if i == 0 {
			assert.Equal(t, log.WarnLevel, e.Level)
			assert.Equal(t, "Transaction ID error in augmenting annotations with concept data: Generated a new transaction ID", e.Message)
			tid = e.Data[tidUtils.TransactionIDKey].(string)
			assert.NotEmpty(t, tid)
		} else {
			assert.Equal(t, tid, e.Data[tidUtils.TransactionIDKey])
		}
	}

	conceptsSearchAPI.AssertExpectations(t)
}

func TestAugmentAnnotationsConceptSearchError(t *testing.T) {
	conceptsSearchAPI := new(ConceptSearchAPIMock)
	ctx := tidUtils.TransactionAwareContext(context.Background(), tidUtils.NewTransactionID())
	conceptsSearchAPI.
		On("SearchConcepts", ctx, testConceptIDs).
		Return(map[string]concept.Concept{}, errors.New("one minute to midnight"))
	a := NewAugmenter(conceptsSearchAPI)

	_, err := a.AugmentAnnotations(ctx, testCanonicalizedAnnotations)

	assert.Error(t, err)

	conceptsSearchAPI.AssertExpectations(t)
}

func TestAugmentAnnotationsWithInvalidConceptID(t *testing.T) {
	conceptsSearchAPI := new(ConceptSearchAPIMock)
	ctx := tidUtils.TransactionAwareContext(context.Background(), tidUtils.NewTransactionID())
	conceptsSearchAPI.
		On("SearchConcepts", ctx, testConceptIDs).
		Return(testConcepts, nil)
	a := NewAugmenter(conceptsSearchAPI)

	testCanonicalizedAnnotations = append(testCanonicalizedAnnotations, Annotation{ConceptId: "xyz"})
	annotations, err := a.AugmentAnnotations(ctx, testCanonicalizedAnnotations)

	assert.NoError(t, err)
	assert.Equal(t, len(expectedAugmentedAnnotations), len(annotations))
	for _, expected := range expectedAugmentedAnnotations {
		assert.Contains(t, annotations, expected)
	}

	conceptsSearchAPI.AssertExpectations(t)
}

type ConceptSearchAPIMock struct {
	mock.Mock
}

func (m *ConceptSearchAPIMock) SearchConcepts(ctx context.Context, ids []string) (map[string]concept.Concept, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(map[string]concept.Concept), args.Error(1)
}

func (m *ConceptSearchAPIMock) GTG() error {
	args := m.Called()
	return args.Error(0)
}

func (m *ConceptSearchAPIMock) Endpoint() string {
	args := m.Called()
	return args.String(0)
}
