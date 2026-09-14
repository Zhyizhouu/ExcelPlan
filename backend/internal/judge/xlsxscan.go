package judge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/xuri/excelize/v2"
)

// sheetFormulas is what the saved XML says about formulas on one sheet.
//
// Read straight from the file because excelize does not expose which cells a
// spilling formula fills. Without that, every UNIQUE or FILTER result would
// look typed in.
type sheetFormulas struct {
	cells  map[string]bool // cells holding a formula of their own
	arrays []area          // ranges filled by an array or spilled formula
	texts  []string
}

type area struct{ c1, r1, c2, r2 int }

func (a area) contains(col, row int) bool {
	return col >= a.c1 && col <= a.c2 && row >= a.r1 && row <= a.r2
}

// computed reports whether a cell's value comes from a formula.
func (s sheetFormulas) computed(col, row int) bool {
	if name, err := excelize.CoordinatesToCellName(col, row); err == nil && s.cells[name] {
		return true
	}
	for _, a := range s.arrays {
		if a.contains(col, row) {
			return true
		}
	}
	return false
}

func parseArea(ref string) (area, bool) {
	from, to, found := strings.Cut(ref, ":")
	if !found {
		to = from
	}
	c1, r1, err1 := excelize.CellNameToCoordinates(from)
	c2, r2, err2 := excelize.CellNameToCoordinates(to)
	if err1 != nil || err2 != nil {
		return area{}, false
	}
	return area{min(c1, c2), min(r1, r2), max(c1, c2), max(r1, r2)}, true
}

// scanFormulas maps each sheet name to its formulas.
func scanFormulas(data []byte) (map[string]sheetFormulas, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	parts := map[string]*zip.File{}
	for _, f := range zr.File {
		parts[f.Name] = f
	}

	sheets, err := sheetParts(parts)
	if err != nil {
		return nil, err
	}
	out := make(map[string]sheetFormulas, len(sheets))
	for name, part := range sheets {
		zf := parts[part]
		if zf == nil {
			continue
		}
		sf, err := readSheetFormulas(zf)
		if err != nil {
			return nil, fmt.Errorf("sheet %q: %w", name, err)
		}
		out[name] = sf
	}
	return out, nil
}

// sheetParts maps sheet names to their XML part, through the workbook's
// relationships, since sheet3.xml is not necessarily the third tab.
func sheetParts(parts map[string]*zip.File) (map[string]string, error) {
	wb, rels := parts["xl/workbook.xml"], parts["xl/_rels/workbook.xml.rels"]
	if wb == nil || rels == nil {
		return nil, fmt.Errorf("not a workbook: missing workbook parts")
	}

	targets := map[string]string{}
	err := walkXML(rels, func(_ *xml.Decoder, se xml.StartElement) error {
		if se.Name.Local == "Relationship" {
			targets[attr(se, "Id")] = attr(se, "Target")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := map[string]string{}
	err = walkXML(wb, func(_ *xml.Decoder, se xml.StartElement) error {
		if se.Name.Local != "sheet" {
			return nil
		}
		target := targets[attr(se, "id")]
		if target == "" {
			return nil
		}
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(target, "/")
		} else {
			target = path.Join("xl", target)
		}
		out[attr(se, "name")] = target
		return nil
	})
	return out, err
}

func readSheetFormulas(zf *zip.File) (sheetFormulas, error) {
	sf := sheetFormulas{cells: map[string]bool{}}
	var cell string
	err := walkXML(zf, func(dec *xml.Decoder, se xml.StartElement) error {
		switch se.Name.Local {
		case "c":
			cell = attr(se, "r")
		case "f":
			sf.cells[cell] = true
			if attr(se, "t") == "array" {
				if a, ok := parseArea(attr(se, "ref")); ok {
					sf.arrays = append(sf.arrays, a)
				}
			}
			var text string
			if err := dec.DecodeElement(&text, &se); err != nil {
				return err
			}
			if text = strings.TrimSpace(text); text != "" {
				sf.texts = append(sf.texts, text)
			}
		}
		return nil
	})
	return sf, err
}

func walkXML(zf *zip.File, visit func(*xml.Decoder, xml.StartElement) error) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if se, ok := tok.(xml.StartElement); ok {
			if err := visit(dec, se); err != nil {
				return err
			}
		}
	}
}

func attr(se xml.StartElement, local string) string {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
