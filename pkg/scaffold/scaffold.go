package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed all:skeletons
var skeletonFS embed.FS

type Template struct {
	ID          string
	Name        string
	Description string
}

type Params struct {
	Name     string
	Registry string
	Domain   string
}

var Templates = []Template{
	{ID: "nextjs-fastapi", Name: "Next.js + FastAPI", Description: "Full-stack web app — Python backend, React frontend"},
	{ID: "static-site", Name: "Static Site", Description: "Single container — nginx static files"},
}

func RenderSkeleton(templateID string, params Params, outDir string) error {
	root := filepath.Join("skeletons", templateID)

	return fs.WalkDir(skeletonFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(outDir, strings.TrimSuffix(relPath, ".tmpl"))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		content, err := skeletonFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}

		tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", path, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, params); err != nil {
			return fmt.Errorf("render template %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("create directory for %s: %w", destPath, err)
		}

		perm := os.FileMode(0644)
		if strings.HasSuffix(destPath, ".sh") {
			perm = 0755
		}

		return os.WriteFile(destPath, buf.Bytes(), perm)
	})
}
