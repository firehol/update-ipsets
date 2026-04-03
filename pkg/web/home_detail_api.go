package web

import (
	"errors"
	"net"
	"net/http"
	pathpkg "path"
	"strconv"
	"strings"

	"github.com/firehol/update-ipsets/pkg/engine"
)

// handleClientIP serves /api/v1/client-ip. The payload is intentionally
// tiny because the homepage only needs an IPv4 bootstrap value when it is
// available from the current request context.
func handleClientIP(resolver *clientIPResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		ip := strings.TrimSpace(resolver.clientIP(r))
		parsed := net.ParseIP(ip)
		if parsed == nil {
			writeJSON(w, http.StatusOK, map[string]string{"ip": ""})
			return
		}
		v4 := parsed.To4()
		if v4 == nil {
			writeJSON(w, http.StatusOK, map[string]string{"ip": ""})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ip": v4.String()})
	}
}

// handleCountryIndex serves /api/v1/countries from the published artifact.
func handleCountryIndex(eng *engine.Engine, cache *fileCache, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := pathpkg.Join("countries", "index.json")
		if cache != nil && cache.ServeRootedFile(w, r, outputDir, rel, "application/json") {
			eng.ObserveCounter("http.entity_artifact.country_index_hit", 1, 0)
			return
		}
		eng.ObserveCounter("http.entity_artifact.country_index_miss", 1, 0)
		apiNoCache(w)
		if exists, readable := rootedRegularFileStatus(outputDir, rel); exists && !readable {
			jsonError(w, http.StatusServiceUnavailable, errors.New("country index artifact is not readable"))
			return
		}
		jsonError(w, http.StatusServiceUnavailable, errors.New("country index artifact is not ready"))
	}
}

// handleCountryDetail serves /api/v1/countries/{code} from the published
// artifact. The code is the final segment of the URL path; any capitalization
// is accepted and normalized to upper-case for artifact lookup.
func handleCountryDetail(eng *engine.Engine, cache *fileCache, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimPrefix(r.URL.Path, "/api/v1/countries/")
		code = strings.TrimSpace(strings.Trim(code, "/"))
		if code == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing country code"})
			return
		}
		code, ok := normalizeCountryCode(code)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "country code must be two ASCII letters"})
			return
		}
		rel := pathpkg.Join("countries", code+".json")
		if cache != nil && cache.ServeRootedFile(w, r, outputDir, rel, "application/json") {
			eng.ObserveCounter("http.entity_artifact.country_detail_hit", 1, 0)
			return
		}
		eng.ObserveCounter("http.entity_artifact.country_detail_miss", 1, 0)
		apiNoCache(w)
		if exists, readable := rootedRegularFileStatus(outputDir, rel); !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "no feeds attribute IPs to this country",
				"code":  code,
			})
			return
		} else if exists && !readable {
			jsonError(w, http.StatusServiceUnavailable, errors.New("country detail artifact is not readable"))
			return
		}
		jsonError(w, http.StatusServiceUnavailable, errors.New("country detail artifact is not readable"))
	}
}

func normalizeCountryCode(raw string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if len(code) != 2 {
		return "", false
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return "", false
		}
	}
	return code, true
}

// handleASNIndex serves /api/v1/asns from the published artifact.
func handleASNIndex(eng *engine.Engine, cache *fileCache, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := pathpkg.Join("asns", "index.json")
		if cache != nil && cache.ServeRootedFile(w, r, outputDir, rel, "application/json") {
			eng.ObserveCounter("http.entity_artifact.asn_index_hit", 1, 0)
			return
		}
		eng.ObserveCounter("http.entity_artifact.asn_index_miss", 1, 0)
		apiNoCache(w)
		if exists, readable := rootedRegularFileStatus(outputDir, rel); exists && !readable {
			jsonError(w, http.StatusServiceUnavailable, errors.New("ASN index artifact is not readable"))
			return
		}
		jsonError(w, http.StatusServiceUnavailable, errors.New("ASN index artifact is not ready"))
	}
}

// handleASNDetail serves /api/v1/asns/{asn} from the published artifact. The
// ASN is parsed as an unsigned integer; anything else returns 400.
func handleASNDetail(eng *engine.Engine, cache *fileCache, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.URL.Path, "/api/v1/asns/")
		raw = strings.TrimSpace(strings.Trim(raw, "/"))
		raw = strings.TrimPrefix(strings.ToUpper(raw), "AS")
		if raw == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing ASN"})
			return
		}
		number, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ASN must be a positive integer"})
			return
		}
		rel := pathpkg.Join("asns", strconv.FormatUint(number, 10)+".json")
		if cache != nil && cache.ServeRootedFile(w, r, outputDir, rel, "application/json") {
			eng.ObserveCounter("http.entity_artifact.asn_detail_hit", 1, 0)
			return
		}
		eng.ObserveCounter("http.entity_artifact.asn_detail_miss", 1, 0)
		apiNoCache(w)
		if exists, readable := rootedRegularFileStatus(outputDir, rel); !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "no feeds attribute IPs to this ASN",
				"asn":   strconv.FormatUint(number, 10),
			})
			return
		} else if exists && !readable {
			jsonError(w, http.StatusServiceUnavailable, errors.New("ASN detail artifact is not readable"))
			return
		}
		jsonError(w, http.StatusServiceUnavailable, errors.New("ASN detail artifact is not readable"))
	}
}

// handleMaintainerIndex serves /api/v1/maintainers. The optional
// categories query parameter narrows the index to maintainers with at
// least one feed in the selected categories.
func handleMaintainerIndex(eng *engine.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		raw := strings.TrimSpace(r.URL.Query().Get("categories"))
		var categories []string
		if raw != "" {
			categories = parseList(raw)
		}
		payload, err := eng.MaintainerIndex(categories)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

// handleMaintainerDetail serves /api/v1/maintainers/{slug}. The slug
// is the final segment of the path and must match the slug computed
// from the maintainer display name by maintainerSlugify.
func handleMaintainerDetail(eng *engine.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiNoCache(w)
		slug := strings.TrimPrefix(r.URL.Path, "/api/v1/maintainers/")
		slug = strings.TrimSpace(strings.Trim(slug, "/"))
		if slug == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing maintainer slug"})
			return
		}
		payload, err := eng.MaintainerDetail(slug)
		if err != nil {
			if errors.Is(err, engine.ErrMaintainerNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			jsonError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}
