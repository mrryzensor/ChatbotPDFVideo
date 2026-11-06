package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type PDFService struct{}

func NewPDFService() *PDFService {
	return &PDFService{}
}

// MergePDFs merges multiple PDF files into one output file
func (s *PDFService) MergePDFs(inFiles []string, outFile string) error {
	if len(inFiles) < 2 {
		return fmt.Errorf("se necesitan al menos 2 archivos PDF para unir")
	}

	conf := model.NewDefaultConfiguration()
	return api.MergeCreateFile(context.Background(), inFiles, outFile, false, conf)
}

// SplitPDF splits a PDF file into individual pages or groups
func (s *PDFService) SplitPDF(inFile string, outDir string, span int) ([]string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, err
	}

	conf := model.NewDefaultConfiguration()
	err := api.SplitFile(context.Background(), inFile, outDir, span, conf)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pdf" {
			results = append(results, filepath.Join(outDir, entry.Name()))
		}
	}
	return results, nil
}

// CompressPDF optimizes a PDF to reduce its file size
func (s *PDFService) CompressPDF(inFile string, outFile string) error {
	conf := model.NewDefaultConfiguration()
	conf.Optimize = true
	conf.OptimizeDuplicateContentStreams = true

	return api.OptimizeFile(context.Background(), inFile, outFile, conf, nil)
}
