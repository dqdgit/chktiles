package main

import (
	"fmt"
	"os"
	"io"
	"strings"
	"regexp"
	"strconv"
	"encoding/xml"
	"path/filepath"
	"github.com/pborman/getopt/v2"
	"github.com/trustmaster/go-aspell"
  "github.com/antchfx/xmlquery"
)

const svgNs = "http://www.w3.org/2000/svg"
const svgDcNs = "http://purl.org/dc/elements/1.1/"
const svgCcNs = "http://creativecommons.org/ns#"
const svgRdfNs = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"

const pxPerIn = 96
const pxPerMm = (0.039370787 * pxPerIn)
const pxPerPt = (0.0138888889 * pxPerIn)
const pxPerPc = (0.1666666667 * pxPerIn)
const pxPerFt = (pxPerIn * 12)
const pxPerCm = (0.3937007874 * pxPerIn)
const pxPerM = (0.0254 * pxPerIn)

const minWidth = 80
const minHeight = 80

type Bag struct {
	Items []string `xml:"li"` 
}

type Subject struct {
	XMLName xml.Name
	Bag Bag `xml:"Bag"`
}

type Work struct {
	XMLName xml.Name
	About string `xml:"about,attr"`
	Subject Subject `xml:"subject"`
	Identifier string `xml:"identifier"`
}

type Rdf struct {
	XMLName xml.Name
	Work Work `xml:"Work"`
}

type Metadata struct {
	XMLName xml.Name
	Id string `xml:"id,attr"`
	Rdf Rdf `xml:"RDF"`
}

type Tspan struct {
	XMLName xml.Name
	Id string `xml:"id,attr"`
	X string `xml:"x,attr"`
	Y string `xml:"y,attr"`
	InnerText string `xml:",innerxml"`
}

type Text struct {
	XMLName xml.Name
	Tspans []Tspan `xml:"tspan"`
}

type Svg struct {
	Width string `xml:"width,attr"`
	Height string `xml:"height,attr"`
	ViewBox string `xml:"viewBox,attr"`
	Metadata Metadata `xml:"metadata"`
	Texts []Text `xml:"text"`
	GroupedTexts []Text `xml:"any>text"`
}

var helpFlag bool
var verboseFlag bool

func toFloat(s string) float64 {
	re := regexp.MustCompile(`[^0-9\.]`)
	f, err := strconv.ParseFloat(re.ReplaceAllString(s, ""), 64)
	if err != nil {
		fmt.Printf("toFloat: ERROR: unable to convert %q, %v\n", s, err)
	}
	return f
}

func getUnitConversion(value string) float64 {
	if strings.HasSuffix(value, "in") {
		return pxPerIn
	} else if strings.HasSuffix(value, "mm") {
		return pxPerMm
	} else if strings.HasSuffix(value, "pt") {
		return pxPerPt
	} else if strings.HasSuffix(value, "pc") {
		return pxPerPc
	} else if strings.HasSuffix(value, "ft") {
		return pxPerFt
	} else if strings.HasSuffix(value, "cm") {
		return pxPerCm
	} else if strings.HasSuffix(value, "m") {
		return pxPerM
	} 
	
	return 1.0
}

func init() {
	getopt.Flag(&helpFlag, '?', "display help")
	getopt.Flag(&verboseFlag, 'v', "output additional information")
}

func usage() {
	fmt.Printf("Usage: %s [-?] [-v] <check-directory> <duplicate-directory>\n", filepath.Base(os.Args[0]))
	fmt.Printf("    -?                         display this help message\n")
	fmt.Printf("    -v                         output additional execution information\n")
	fmt.Printf("    <check-directory>          path to the directory tree to check\n")
	fmt.Printf("    <duplication-directory>    path to the directory tree to look for duplicates\n")
}

func printSvg(svg Svg) {
	fmt.Printf("width: %q, height: %q, viewBox: %q\n", svg.Width, svg.Height, svg.ViewBox)
	keywords := strings.Join(svg.Metadata.Rdf.Work.Subject.Bag.Items, ", ")
	fmt.Printf("keywords: %s\n", keywords)
	fmt.Printf("identifier: %s\n", svg.Metadata.Rdf.Work.Identifier)
	//fmt.Println(svg)
}

func parseSvg(reader io.Reader) (*Svg , error) {
	svg := &Svg{}

	decoder := xml.NewDecoder(reader)
	if err := decoder.Decode(svg); err != nil {
		fmt.Printf("parseSvg: \tERROR: could not parse svg %v\n", err)
    return nil, err
	}

	return svg, nil
}

func checkKeywords(path string, svg Svg) {
  if len(svg.Metadata.Rdf.Work.Subject.Bag.Items) == 0 {
		fmt.Printf("%q: \tERROR: keywords missing\n", path)
	}
}

func checkSize(path string, svg Svg) {
	w := toFloat(svg.Width)
	h := toFloat(svg.Height)
	if w < minWidth {
		fmt.Printf("%q: \tERROR: width (%f) is too small\n", path, w)
	}

	if h < minHeight {
		fmt.Printf("%q: \tERROR: height (%f) is too small\n", path, h)
	}
}

func checkUnits(path string, svg Svg) {
	if u := getUnitConversion(svg.Width); u != 1.0 {
		fmt.Printf("%q: \tWARNING: width units are not px, %q\n", path, svg.Width)
	}

	if u := getUnitConversion(svg.Height); u != 1.0 {
		fmt.Printf("%q: \tWARNING: height units are not px, %q\n", path, svg.Height)
	}
}

func checkIdentifier(path string, svg Svg) {
  if svg.Metadata.Rdf.Work.Identifier == "" {
		fmt.Printf("%q: \tERROR: Identifier missing\n", path)
	}
}

func checkKeywordSpelling(path string, svg Svg) {
	speller, err := aspell.NewSpeller(map[string]string{"lang": "en_US,"})
	if err != nil {
		fmt.Printf("checkKeywordSpelling: ERROR: %v\n", err)
		return
	}
	defer speller.Delete()
	
	keywords := svg.Metadata.Rdf.Work.Subject.Bag.Items
	if len(keywords) == 0 {
		return
	}

	var misspelled []string
	for _, keyword := range keywords {
		if keyword != "" {
			words := strings.Split(keyword, " ")
			for _, word := range words {
				if !speller.Check(word) {
					misspelled = append(misspelled, word)
				}
			}
		}
	}

	if len(misspelled) > 0 {
		s := strings.Join(misspelled, ", ")
		fmt.Printf("%q: \tERROR: keywords misspelled: %s\n", path, s)
	}
}

func checkTspanSpelling(path string, svg Svg) {
	texts := svg.Texts
	fmt.Printf("%q texts: %d, grouped texts: %d\n", path, len(texts), len(svg.GroupedTexts))
}

func checkTiles(checkDir string, dupDir string) error {
	err := filepath.Walk(checkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("checkTiles: \tERROR: unable to access path %q, %v\n", path, err)
			return err
		}

		if filepath.Ext(path) != ".svg" {
			return nil
		}

		if verboseFlag {
			fmt.Printf("checkTiles: %q\n", path)
		}

		file, err := os.Open(path)
		if err != nil {
			fmt.Printf("checkTiles: \tERROR: unable to open %q, %v\n", path, err)
			return err
		}
		defer file.Close()

		svg, err := parseSvg(file)
		if err != nil {
			return err
		}

		if verboseFlag {
			printSvg(*svg)
		}

		checkKeywords(path, *svg)
		checkSize(path, *svg)
		checkUnits(path, *svg)
		checkIdentifier(path, *svg)
		checkKeywordSpelling(path, *svg)
		checkTspanSpelling(path, *svg)

		return nil
	})

	if err != nil {
		fmt.Printf("checkTiles: \tERROR: unable to walk directory %q, %v\n", checkDir, err)
	}

	return err
}

func main() {
	getopt.Parse()

	if helpFlag {
		usage()
		os.Exit(0)
	}

	if verboseFlag {
		fmt.Printf("nArgs: %d, Args: %s\n", len(os.Args), strings.Join(os.Args, ", "))
	}

	args := getopt.Args()
	if len(args) < 2 {
		usage()
		os.Exit(1)
	}

	checkTiles(args[0], args[1])

	os.Exit(0)
}
