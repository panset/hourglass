// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Handler is the control API (plain JSON over HTTP).
func (s *Sim) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /pools", func(w http.ResponseWriter, r *http.Request) {
		names, err := s.ListPools(r.Context())
		if err != nil {
			s.fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pools": names})
	})
	mux.HandleFunc("POST /pools", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &in) {
			return
		}
		if strings.TrimSpace(in.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		created, err := s.CreatePool(r.Context(), in.Name)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		code := http.StatusOK
		if created {
			code = http.StatusCreated
		}
		writeJSON(w, code, map[string]any{"name": in.Name, "created": created})
	})
	mux.HandleFunc("DELETE /pools/{name}", func(w http.ResponseWriter, r *http.Request) {
		err := s.DeletePool(r.Context(), r.PathValue("name"))
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "pool not found"})
			return
		}
		if err != nil {
			s.fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /pressure", func(w http.ResponseWriter, r *http.Request) {
		var in Pressure
		if !decode(w, r, &in) {
			return
		}
		if err := s.SetPressure(r.Context(), in); err != nil {
			if strings.Contains(err.Error(), "between 0 and 1") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			s.fail(w, r, err)
			return
		}
		s.writePressure(w, r)
	})
	mux.HandleFunc("GET /pressure", s.writePressure)
	return mux
}

func (s *Sim) writePressure(w http.ResponseWriter, r *http.Request) {
	cpu, mem, st, err := s.ReadPressure(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]float64{"cpu": cpu, "memory": mem, "storage": st})
}

func (s *Sim) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("control request failed", "method", r.Method, "path", r.URL.Path, "err", err.Error())
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
