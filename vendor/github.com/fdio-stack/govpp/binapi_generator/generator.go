package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/bennyscetbun/jsongo"
)

const (
	inputFileExt = ".json" // filename extension of files that should be processed as the input
)

// context is a structure storing details of a particular code generation task
type context struct {
	inputFile   string            // file with input JSON data
	outputFile  string            // file with output data
	packageName string            // name of the Go package being generated
	packageDir  string            // directory where the package source files are located
	types       map[string]string // map of the VPP typedef names to generated Go typedef names
}

func main() {
	inputFile := flag.String("input-file", "", "Input JSON file.")
	inputDir := flag.String("input-dir", ".", "Input directory with JSON files.")
	outputDir := flag.String("output-dir", ".", "Output directory where package folders will be generated.")
	flag.Parse()

	if *inputFile == "" && *inputDir == "" {
		fmt.Fprintln(os.Stderr, "ERROR: input-file or input-dir must be specified")
		os.Exit(1)
	}

	var err, tmpErr error
	if *inputFile != "" {
		// process one input file
		err = generateFromFile(*inputFile, *outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: code generation from %s failed: %v\n", *inputFile, err)
		}
	} else {
		// process all files in specified directory
		files, err := getInputFiles(*inputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: code generation failed: %v\n", err)
		}
		for _, file := range files {
			tmpErr = generateFromFile(file, *outputDir)
			if tmpErr != nil {
				fmt.Fprintf(os.Stderr, "ERROR: code generation from %s failed: %v\n", file, err)
				err = tmpErr // remember that the error occurred
			}
		}
	}
	if err != nil {
		os.Exit(1)
	}
}

// getInputFiles returns all input files located in specified directory
func getInputFiles(inputDir string) ([]string, error) {
	files, err := ioutil.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s failed: %v", inputDir, err)
	}
	res := make([]string, 0)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), inputFileExt) {
			res = append(res, inputDir+"/"+f.Name())
		}
	}
	return res, nil
}

// generateFromFile generates Go bindings from one input JSON file
func generateFromFile(inputFile, outputDir string) error {
	ctx, err := getContext(inputFile, outputDir)
	if err != nil {
		return err
	}

	// read JSON file
	jsonRoot, err := readJSON(inputFile)
	if err != nil {
		return err
	}

	// create output directory
	err = os.MkdirAll(ctx.packageDir, 0777)
	if err != nil {
		return fmt.Errorf("creating output directory %s failed: %v", ctx.packageDir, err)
	}

	// open output file
	f, err := os.Create(ctx.outputFile)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("creating output file %s failed: %v", ctx.outputFile, err)
	}
	w := bufio.NewWriter(f)

	// generate Go package code
	err = generatePackage(ctx, w, jsonRoot)
	if err != nil {
		return err
	}

	// go format the output file (non-fatal if fails)
	exec.Command("gofmt", "-w", ctx.outputFile).Run()

	return nil
}

// getContext returns context details of the code generation task
func getContext(inputFile, outputDir string) (*context, error) {
	if !strings.HasSuffix(inputFile, inputFileExt) {
		return nil, fmt.Errorf("invalid input file name %s", inputFile)
	}

	ctx := &context{inputFile: inputFile}
	inputFileName := filepath.Base(inputFile)

	ctx.packageName = inputFileName[0:strings.Index(inputFileName, ".")]
	if ctx.packageName == "interface" {
		// 'interface' cannot be a package name, it is a go keyword
		ctx.packageName = "interfaces"
	}

	ctx.packageDir = outputDir + "/" + ctx.packageName + "/"
	ctx.outputFile = ctx.packageDir + ctx.packageName + ".go"

	return ctx, nil
}

// readJSON parses a JSON file into memory
func readJSON(inputFile string) (*jsongo.JSONNode, error) {
	root := jsongo.JSONNode{}

	inputData, err := ioutil.ReadFile(inputFile)
	if err != nil {
		return nil, fmt.Errorf("reading from JSON file failed: %v", err)
	}

	err = json.Unmarshal(inputData, &root)
	if err != nil {
		return nil, fmt.Errorf("JSON unmarshall failed: %v", err)
	}

	return &root, nil

}

// generatePackage generates Go code of a package from provided JSON
func generatePackage(ctx *context, w *bufio.Writer, jsonRoot *jsongo.JSONNode) error {
	// generate file header
	generatePackageHeader(ctx, w, jsonRoot)

	// generate data types
	ctx.types = make(map[string]string)
	types := jsonRoot.Map("types")
	for i := 0; i < types.Len(); i++ {
		typ := types.At(i)
		err := generateMessage(ctx, w, typ, true)
		if err != nil {
			return err
		}
	}

	// generate messages
	messages := jsonRoot.Map("messages")
	for i := 0; i < messages.Len(); i++ {
		msg := messages.At(i)
		err := generateMessage(ctx, w, msg, false)
		if err != nil {
			return err
		}
	}

	// flush the data:
	err := w.Flush()
	if err != nil {
		return fmt.Errorf("flushing data to %s failed: %v", ctx.outputFile, err)
	}

	return nil
}

// generateMessage generates Go code of one VPP message encoded in JSON into provided writer
func generateMessage(ctx *context, w io.Writer, msg *jsongo.JSONNode, isType bool) error {
	if msg.Len() == 0 || msg.At(0).GetType() != jsongo.TypeValue {
		return errors.New("invalid JSON for message specified")
	}

	msgName, ok := msg.At(0).Get().(string)
	if !ok {
		return fmt.Errorf("invalid JSON for message specified, message name is %T, not a string", msg.At(0).Get())
	}
	structName := camelCaseName(strings.Title(msgName))

	// generate struct fields into the slice
	fields := make([]string, 0)
	for j := 0; j < msg.Len(); j++ {
		if jsongo.TypeArray == msg.At(j).GetType() {
			fld := msg.At(j)
			err := processMessageField(ctx, &fields, fld)
			if err != nil {
				return err
			}
		}
	}

	// generate struct comment
	generateMessageComment(w, structName, msgName, isType)

	// generate struct header
	fmt.Fprintln(w, "type", structName, "struct {")

	// print out the fields
	for _, field := range fields {
		fmt.Fprintln(w, field)
	}

	// generate end of the struct
	fmt.Fprintln(w, "}")

	// generate name getter
	if isType {
		generateTypeNameGetter(w, structName, msgName)
	} else {
		generateMessageNameGetter(w, structName, msgName)
	}

	// generate CRC getter
	crcIf := msg.At(msg.Len() - 1).At("crc").Get()
	if crc, ok := crcIf.(string); ok {
		generateCrcGetter(w, structName, crc)
	}

	// if this is a type, save it in the map for later use
	if isType {
		ctx.types[fmt.Sprintf("vl_api_%s_t", msgName)] = structName
	}

	return nil
}

// processMessageField process JSON describing one message field into Go code emitted into provided slice of message fields
func processMessageField(ctx *context, fields *[]string, fld *jsongo.JSONNode) error {
	if fld.Len() < 2 || fld.At(0).GetType() != jsongo.TypeValue || fld.At(1).GetType() != jsongo.TypeValue {
		return errors.New("invalid JSON for message field specified")
	}

	fieldVppType, ok := fld.At(0).Get().(string)
	if !ok {
		return fmt.Errorf("invalid JSON for message specified, field type is %T, not a string", fld.At(0).Get())
	}
	fieldName, ok := fld.At(1).Get().(string)
	if !ok {
		return fmt.Errorf("invalid JSON for message specified, field name is %T, not a string", fld.At(1).Get())
	}

	// skip internal fields
	fieldNameLower := strings.ToLower(fieldName)
	if fieldNameLower == "crc" || fieldNameLower == "_vl_msg_id" || fieldNameLower == "client_index" || fieldNameLower == "context" {
		return nil
	}

	fieldName = strings.TrimPrefix(fieldName, "_")
	fieldName = camelCaseName(strings.Title(fieldName))

	fieldStr := ""
	isArray := false
	arrayDimension := 0

	fieldStr += "\t" + fieldName + " "
	if fld.Len() > 2 {
		isArray = true
		arrayDimension = int(fld.At(2).Get().(float64))
		fieldStr += "[]"
	}

	dataType := translateVppType(ctx, fieldVppType, isArray)
	fieldStr += dataType

	if isArray {
		if arrayDimension == 0 {
			// write to previous one
			(*fields)[len(*fields)-1] += fmt.Sprintf("\t`struc:\"sizeof=%s\"`", fieldName)
		} else {
			fieldStr += fmt.Sprintf("\t`struc:\"[%d]%s\"`", arrayDimension, dataType)
		}
	}

	*fields = append(*fields, fieldStr)
	return nil
}

// generatePackageHeader generates package header into provider writer
func generatePackageHeader(ctx *context, w io.Writer, rootNode *jsongo.JSONNode) {
	fmt.Fprintln(w, "// Package "+ctx.packageName+" provides the Go interface to VPP binary API of the "+ctx.packageName+" VPP module.")
	fmt.Fprintln(w, "// Generated from '"+filepath.Base(ctx.inputFile)+"' on "+time.Now().Format(time.RFC1123)+".")

	fmt.Fprintln(w, "package "+ctx.packageName)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "// VlApiVersion contains version of the API.")
	vlAPIVersion := rootNode.Map("vl_api_version")
	if vlAPIVersion != nil {
		fmt.Fprintln(w, "const VlAPIVersion = ", vlAPIVersion.Get())
	}
	fmt.Fprintln(w)
}

// generateMessageComment generates comment for a message into provider writer
func generateMessageComment(w io.Writer, structName string, msgName string, isType bool) {
	fmt.Fprintln(w)
	if isType {
		fmt.Fprintln(w, "// "+structName+" is the Go representation of the VPP binary API data type '"+msgName+"'.")
	} else {
		fmt.Fprintln(w, "// "+structName+" is the Go representation of the VPP binary API message '"+msgName+"'.")
	}
}

// generateMessageNameGetter generates getter for original VPP message name into provider writer
func generateMessageNameGetter(w io.Writer, structName string, msgName string) {
	fmt.Fprintln(w, "func (*"+structName+") GetMessageName() string {")
	fmt.Fprintln(w, "\treturn \""+msgName+"\"")
	fmt.Fprintln(w, "}")
}

// generateTypeNameGetter generates getter for original VPP type name into provider writer
func generateTypeNameGetter(w io.Writer, structName string, msgName string) {
	fmt.Fprintln(w, "func (*"+structName+") GetTypeName() string {")
	fmt.Fprintln(w, "\treturn \""+msgName+"\"")
	fmt.Fprintln(w, "}")
}

// generateCrcGetter generates getter for CRC checksum of the message definition into provider writer
func generateCrcGetter(w io.Writer, structName string, crc string) {
	crc = strings.TrimPrefix(crc, "0x")
	fmt.Fprintln(w, "func (*"+structName+") GetCrcString() string {")
	fmt.Fprintln(w, "\treturn \""+crc+"\"")
	fmt.Fprintln(w, "}")
}

// translateVppType translates the VPP data type into Go data type
func translateVppType(ctx *context, vppType string, isArray bool) string {
	// basic types
	switch vppType {
	case "u8":
		if isArray {
			return "byte"
		}
		return "uint8"
	case "i8":
		return "int8"
	case "u16":
		return "uint16"
	case "i16":
		return "int16"
	case "u32":
		return "uint32"
	case "i32":
		return "int32"
	case "u64":
		return "uint64"
	case "i64":
		return "int64"
	case "f64":
		return "float64"
	}

	// typedefs
	typ, ok := ctx.types[vppType]
	if ok {
		return typ
	}

	panic(fmt.Sprintf("Unknown VPP type %s", vppType))
}

// camelCaseName returns correct name identifier (camelCase).
func camelCaseName(name string) (should string) {
	// Fast path for simple cases: "_" and all lowercase.
	if name == "_" {
		return name
	}
	allLower := true
	for _, r := range name {
		if !unicode.IsLower(r) {
			allLower = false
			break
		}
	}
	if allLower {
		return name
	}

	// Split camelCase at any lower->upper transition, and split on underscores.
	// Check each word for common initialisms.
	runes := []rune(name)
	w, i := 0, 0 // index of start of word, scan
	for i+1 <= len(runes) {
		eow := false // whether we hit the end of a word
		if i+1 == len(runes) {
			eow = true
		} else if runes[i+1] == '_' {
			// underscore; shift the remainder forward over any run of underscores
			eow = true
			n := 1
			for i+n+1 < len(runes) && runes[i+n+1] == '_' {
				n++
			}

			// Leave at most one underscore if the underscore is between two digits
			if i+n+1 < len(runes) && unicode.IsDigit(runes[i]) && unicode.IsDigit(runes[i+n+1]) {
				n--
			}

			copy(runes[i+1:], runes[i+n+1:])
			runes = runes[:len(runes)-n]
		} else if unicode.IsLower(runes[i]) && !unicode.IsLower(runes[i+1]) {
			// lower->non-lower
			eow = true
		}
		i++
		if !eow {
			continue
		}

		// [w,i) is a word.
		word := string(runes[w:i])
		if u := strings.ToUpper(word); commonInitialisms[u] {
			// Keep consistent case, which is lowercase only at the start.
			if w == 0 && unicode.IsLower(runes[w]) {
				u = strings.ToLower(u)
			}
			// All the common initialisms are ASCII,
			// so we can replace the bytes exactly.
			copy(runes[w:], []rune(u))
		} else if w > 0 && strings.ToLower(word) == word {
			// already all lowercase, and not the first word, so uppercase the first character.
			runes[w] = unicode.ToUpper(runes[w])
		}
		w = i
	}
	return string(runes)
}

// commonInitialisms is a set of common initialisms that need to stay in upper case.
var commonInitialisms = map[string]bool{
	"ACL":   true,
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"ICMP":  true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
	"XMPP":  true,
	"XSRF":  true,
	"XSS":   true,
}
