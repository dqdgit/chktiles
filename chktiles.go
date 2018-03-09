package main

import (
	"fmt"
	"os"
	"io"
	"strings"
	"regexp"
	"strconv"
	"path/filepath"
	"crypto/md5"
	"encoding/hex"
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

var helpFlag bool
var verboseFlag bool

func toFloat(s string) float64 {
	re := regexp.MustCompile(`[^0-9\.]`)
	f, err := strconv.ParseFloat(re.ReplaceAllString(s, ""), 64)
	if err != nil {
		fmt.Printf("toFloat\tERROR\tunable to convert %q, %v\n", s, err)
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

func printSvg(node *xmlquery.Node) {
	var n *xmlquery.Node
	n = xmlquery.FindOne(node, "//svg")
	w := n.SelectAttr("width")
	h := n.SelectAttr("height")
	v := n.SelectAttr("viewBox")
	fmt.Printf("  ** Width: %s, Height: %s, Viewbox: %s\n", w, h, v)
}

func parseSvg(reader io.Reader) (*xmlquery.Node , error) {
	xmlDoc, err := xmlquery.Parse(reader)
	if err != nil {
		fmt.Printf("parseSvg: \tERROR\tcould not parse SVG file. %v\n", err)
		return nil, err
	}

	return xmlDoc, nil
}

func checkKeywords(path string, node *xmlquery.Node) {
	var nodes []*xmlquery.Node
	nodes = xmlquery.Find(node, "//rdf:li")
	if len(nodes) == 0 {
		fmt.Printf("%q\tERROR\tKeywords missing\n", path)
	}
}

func checkSize(path string, node *xmlquery.Node) {
	var n *xmlquery.Node
	n = xmlquery.FindOne(node, "//svg")
	w := toFloat(n.SelectAttr("width"))
	h := toFloat(n.SelectAttr("height"))

	if w < minWidth {
		fmt.Printf("%q\tERROR\tWidth (%f) is too small\n", path, w)
	}

	if h < minHeight {
		fmt.Printf("%q\tERROR\tHeight (%f) is too small\n", path, h)
	}
}

func checkUnits(path string, node *xmlquery.Node) {
	var n *xmlquery.Node
	n = xmlquery.FindOne(node, "//svg")
	w := n.SelectAttr("width")
	h := n.SelectAttr("height")

	if u := getUnitConversion(w); u != 1.0 {
		fmt.Printf("%q\tWARNING\tWidth units are not px, %q\n", path, w)
	}

	if u := getUnitConversion(h); u != 1.0 {
		fmt.Printf("%q\tWARNING\tHeight units are not px, %q\n", path, h)
	}	
}

func checkIdentifier(path string, node *xmlquery.Node) {
	var n *xmlquery.Node
	n = xmlquery.FindOne(node, "//dc:identifier")
	if n == nil {
		fmt.Printf("%q\tERROR\tIdentifier missing\n", path)		
	}
}

func checkKeywordSpelling(path string, node *xmlquery.Node) {
	speller, err := aspell.NewSpeller(map[string]string{"lang": "en_US,"})
	if err != nil {
		fmt.Printf("checkKeywordSpelling\tERROR\t%v\n", err)
		return
	}
	defer speller.Delete()

	var nodes []*xmlquery.Node
	nodes = xmlquery.Find(node, "//rdf:li")
	if len(nodes) == 0 {
		return 
	}

	var keywords []string
	for _, n := range nodes {
		keywords = append(keywords, n.InnerText())
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
		fmt.Printf("%q\tERROR\tKeywords misspelled: %s\n", path, s)
	}
}

func checkTspanSpelling(path string, node *xmlquery.Node) {
	speller, err := aspell.NewSpeller(map[string]string{"lang": "en_US,"})
	if err != nil {
		fmt.Printf("checkKeywordSpelling\tERROR\t%v\n", err)
		return
	}
	defer speller.Delete()

	var nodes []*xmlquery.Node
	nodes = xmlquery.Find(node, "//svg:tspan")
	if len(nodes) == 0 {
		return 
	}

	var tspans []string
	for _, n := range nodes {
		tspans = append(tspans, n.InnerText())
	}

	var misspelled []string
	for _, tspan := range tspans {
		if tspan != "" {
			words := strings.Split(tspan, " ")
			for _, word := range words {
				if !speller.Check(strings.Replace(word, "/", "", -1)) {
					misspelled = append(misspelled, word)
				}
			}
		}
	}

	if len(misspelled) > 0 {
		s := strings.Join(misspelled, ", ")
		fmt.Printf("%q\tERROR\tText misspelled: %s\n", path, s)
	}
}

func makeHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("makeHash\tERROR\tunable to open %q, %v\n", path, err)
		return ""
	}
  defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		fmt.Printf("makeHash\tERROR\tunable to create hash of %q, %v\n", path, err)
		return ""
	}

	return hex.EncodeToString(h.Sum(nil))
}

func getFileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		fmt.Print("getFileSize\tERROR\tunable to get size of %q, %v\n", path, err)
		return 0
	}

	return fi.Size()
}

func checkDuplicates(checkPath string, dupDir string, node *xmlquery.Node) {
	aHash := makeHash(checkPath)
	aBasename := filepath.Base(checkPath)
	aSize := getFileSize(checkPath)

	err := filepath.Walk(dupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("checkDuplicates\tERROR\tunable to access %q, %v\n", path, err)
			return err
		}

		if filepath.Ext(path) != ".svg" {
			return nil
		}

		if aBasename == filepath.Base(path) {
			fmt.Printf("%q\tWARNING\tduplicate file name %q\n", checkPath, path)
		}

		if aSize == getFileSize(path) {
			fmt.Printf("%q\tWARNING\tduplicate file size %q\n", checkPath, path)
		}

		if aHash == makeHash(path) {
			fmt.Printf("%q\tWARNING\tduplicate file hash %q\n", checkPath, path)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("checkDuplicates\tERROR\tunable to walk directory %q, %v\n", dupDir, err)
	}
}

func checkTiles(checkDir string, dupDir string) error {
	err := filepath.Walk(checkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("checkTiles\tERROR\tunable to access path %q, %v\n", path, err)
			return err
		}

		if filepath.Ext(path) != ".svg" {
			return nil
		}

		if verboseFlag {
			fmt.Printf("checkTiles%q\n", path)
		}

		file, err := os.Open(path)
		if err != nil {
			fmt.Printf("checkTiles\tERROR\tunable to open %q, %v\n", path, err)
			return err
		}
		defer file.Close()

		rootNode, err := parseSvg(file)
		if err != nil {
			return err
		}

		if verboseFlag {
			printSvg(rootNode)
		}

		checkKeywords(path, rootNode)
		checkSize(path, rootNode)
		checkUnits(path, rootNode)
		checkIdentifier(path, rootNode)
		checkKeywordSpelling(path, rootNode)
		checkTspanSpelling(path, rootNode)
		checkDuplicates(path, dupDir, rootNode)

		return nil
	})

	if err != nil {
		fmt.Printf("checkTiles\tERROR\tunable to walk directory %q, %v\n", checkDir, err)
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
