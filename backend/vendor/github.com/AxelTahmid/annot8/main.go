package annot8

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/go-chi/chi/v5"
)

type GenerateParams struct {
	Router         chi.Router
	Config         Config
	FilePath       string
	RenameFunction ModelNameFunc
}

// GenerateOpenAPISpecFile generates the OpenAPI spec and writes it to the given file path.
func GenerateOpenAPISpecFile(p *GenerateParams) error {
	slog.Debug("[annot8] GenerateOpenAPISpecFile: generating OpenAPI spec", "filePath", p.FilePath)

	ensureTypeIndex()

	renameFunc := p.RenameFunction
	if renameFunc == nil {
		renameFunc = DefaultModelNameFunc
	}

	gen := NewGeneratorWithCache(typeIndex)
	gen.SetModelNameFunc(renameFunc)

	spec := gen.GenerateSpec(p.Router, p.Config)

	slog.Debug("[annot8] GenerateOpenAPISpecFile: writing OpenAPI spec to file", "version", spec.Info.Version)

	file, err := os.Create(p.FilePath)
	if err != nil {
		slog.Error("[annot8] GenerateOpenAPISpecFile: failed to create file", "err", err, "path", p.FilePath)
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err = enc.Encode(spec); err != nil {
		slog.Error("[annot8] GenerateOpenAPISpecFile: failed to write file", "err", err)
		return err
	}

	slog.Debug("[annot8] GenerateOpenAPISpecFile: annot8.json written successfully")
	return nil
}

func SwaggerUIHandler(specURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		htmlContent, err := scalar.ApiReferenceHTML(&scalar.Options{
			SpecURL: specURL,
			CustomOptions: scalar.CustomOptions{
				PageTitle: "API Documentation",
			},
			DarkMode:           true,
			ShowSidebar:        true,
			HideModels:         false,
			HideDownloadButton: false,
			Layout:             scalar.LayoutModern,
		})

		if err != nil {
			slog.Error("[annot8] SwaggerUIHandler: failed to generate API reference HTML", "error", err)
			http.Error(w, "Failed to generate API reference", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, htmlContent)
	}
}
