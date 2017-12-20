package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Financial-Times/draft-annotations-api/annotations"
	"github.com/Financial-Times/draft-annotations-api/mapper"
	tidutils "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	annotationsRW        annotations.RW
	annotationsAPI       annotations.UPPAnnotationsAPI
	c14n                 *annotations.Canonicalizer
	annotationsAugmenter annotations.Augmenter
}

func New(rw annotations.RW, annotationsAPI annotations.UPPAnnotationsAPI, c14n *annotations.Canonicalizer, augmenter annotations.Augmenter) *Handler {
	return &Handler{
		rw,
		annotationsAPI,
		c14n,
		augmenter,
	}
}

func (h *Handler) ReadAnnotations(w http.ResponseWriter, r *http.Request) {
	contentUUID := vestigo.Param(r, "uuid")
	tID := tidutils.GetTransactionIDFromRequest(r)
	ctx := tidutils.TransactionAwareContext(context.Background(), tID)

	readLog := log.WithField(tidutils.TransactionIDKey, tID).WithField("uuid", contentUUID)

	w.Header().Add("Content-Type", "application/json")

	readLog.Info("Reading from annotations RW...")
	rwAnnotations, hash, found, err := h.annotationsRW.Read(ctx, contentUUID)
	if err != nil {
		writeMessage(w, fmt.Sprintf("Annotations RW error: %v", err), http.StatusInternalServerError)
		return
	}

	var rawAnnotations []annotations.Annotation
	var response annotations.Annotations
	if found {
		rawAnnotations = rwAnnotations.Annotations
		w.Header().Set(annotations.DocumentHashHeader, hash)
	} else {
		readLog.Info("Annotations not found: Retrieving annotations from UPP")
		uppResponse, err := h.annotationsAPI.Get(ctx, contentUUID)
		if err != nil {
			readLog.WithError(err).Error("Error in calling UPP Public Annotations API")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer uppResponse.Body.Close()

		if uppResponse.StatusCode != http.StatusOK {
			if uppResponse.StatusCode == http.StatusNotFound || uppResponse.StatusCode == http.StatusBadRequest {
				w.WriteHeader(uppResponse.StatusCode)
				io.Copy(w, uppResponse.Body)
			} else {
				writeMessage(w, "Service unavailable", http.StatusServiceUnavailable)
			}
			return
		}

		respBody, _ := ioutil.ReadAll(uppResponse.Body)
		convertedBody, err := mapper.ConvertPredicates(respBody)
		if err != nil {
			readLog.WithError(err).Error("Error converting predicates from UPP Public Annotations API response")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} else if err == nil && convertedBody == nil {
			writeMessage(w, "No annotations can be found", http.StatusNotFound)
			return
		}

		rawAnnotations = []annotations.Annotation{}
		json.Unmarshal(convertedBody, &rawAnnotations)

		w.WriteHeader(uppResponse.StatusCode)
	}

	readLog.Info("Augmenting annotations...")
	augmentedAnnotations, err := h.annotationsAugmenter.AugmentAnnotations(ctx, rawAnnotations)
	if err != nil {
		writeMessage(w, fmt.Sprintf("Annotations augmenter error: %v", err), http.StatusInternalServerError)
		return
	}
	response = annotations.Annotations{augmentedAnnotations}

	json.NewEncoder(w).Encode(&response)
}

func (h *Handler) WriteAnnotations(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	contentUUID := vestigo.Param(r, "uuid")
	tID := tidutils.GetTransactionIDFromRequest(r)
	ctx := tidutils.TransactionAwareContext(context.Background(), tID)

	oldHash := r.Header.Get(annotations.PreviousDocumentHashHeader)

	writeLog := log.WithField(tidutils.TransactionIDKey, tID).WithField("uuid", contentUUID)

	if err := validateUUID(contentUUID); err != nil {
		writeLog.WithError(err).Error("Invalid content UUID")
		writeMessage(w, fmt.Sprintf("Invalid content UUID: %v", contentUUID), http.StatusBadRequest)
		return
	}

	var draftAnnotations annotations.Annotations
	err := json.NewDecoder(r.Body).Decode(&draftAnnotations)
	if err != nil {
		writeLog.WithError(err).Error("Unable to unmarshal annotations body")
		writeMessage(w, fmt.Sprintf("Unable to unmarshal annotations body: %v", err.Error()), http.StatusBadRequest)
		return
	}

	writeLog.Info("Canonicalizing annotations...")
	draftAnnotations.Annotations = h.c14n.Canonicalize(draftAnnotations.Annotations)

	writeLog.Info("Writing to annotations RW...")
	newHash, err := h.annotationsRW.Write(ctx, contentUUID, &draftAnnotations, oldHash)
	if err != nil {
		writeLog.WithError(err).Error("Error in writing draft annotations")
		writeMessage(w, fmt.Sprintf("Error in writing draft annotations: %v", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set(annotations.DocumentHashHeader, newHash)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(draftAnnotations)
}

func validateUUID(u string) error {
	_, err := uuid.FromString(u)
	return err
}

func writeMessage(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)

	message := make(map[string]interface{})
	message["message"] = msg
	j, err := json.Marshal(&message)

	if err != nil {
		log.WithError(err).Error("Failed to parse provided message to json, this is a bug.")
		return
	}

	w.Write(j)
}
