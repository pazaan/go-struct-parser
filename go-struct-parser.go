package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

type SchemaProperty struct {
	Type string `json:"type,omitempty"`
	Ref  string `json:"$ref,omitempty"`
}

type Schema struct {
	Type       string                 `json:"type,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

func main() {
	argsWithoutProg := os.Args[1:]

	fset := token.NewFileSet() // positions are relative to fset

	// Parse src but stop after processing the imports.
	f, err := parser.ParseFile(fset, argsWithoutProg[0], nil, parser.AllErrors)
	if err != nil {
		fmt.Println(err)
		return
	}

	identType := reflect.TypeOf((*ast.Ident)(nil)).Elem()
	r, _ := regexp.Compile(`json:\s*"(.*?)"`)
	rInherit, _ := regexp.Compile(`bson:\s*",inline"`)

	for _, s := range f.Scope.Objects {
		if s.Kind == ast.Typ {
			fmt.Println("\n" + s.Name)
			d := s.Decl.(*ast.TypeSpec)
			// spew.Dump(d.Type)
			var jsonSchema *Schema
			if reflect.TypeOf(d.Type).Elem() == reflect.TypeOf((*ast.ArrayType)(nil)).Elem() {
				// TODO: Will panic for non-starred elements. ie  `[]*Association` is fine,  `[]Association` is not
				fmt.Println("Array of '#/definitions/" + d.Type.(*ast.ArrayType).Elt.(*ast.StarExpr).X.(*ast.Ident).Name + "'")
				continue
			}
			// Ignore any type that's not a struct (such as interfaces)
			if reflect.TypeOf(d.Type).Elem() != reflect.TypeOf((*ast.StructType)(nil)).Elem() {
				continue
			}
			for _, f := range d.Type.(*ast.StructType).Fields.List {
				if f.Tag != nil {
					if jsonSchema == nil {
						jsonSchema = &Schema{Type: "object"}
						jsonSchema.Properties = make(map[string]interface{})
					}
					matchesInherit := rInherit.FindStringSubmatch(f.Tag.Value)
					if len(matchesInherit) >= 1 {
						baseType := f.Type.(*ast.SelectorExpr)
						fmt.Println("+--> " + baseType.X.(*ast.Ident).String() + "." + baseType.Sel.String())
					}
					matches := r.FindStringSubmatch(f.Tag.Value)
					if len(matches) >= 2 && matches[1] != "-" {
						typeInfo := strings.Split(matches[1], ",")
						if len(typeInfo[0]) == 0 {
							typeInfo[0] = f.Names[0].Name
						}
						typeType := reflect.TypeOf(f.Type).Elem()

						var elementType string
						switch typeType {
						case identType:
							elementType = f.Type.(*ast.Ident).Name
						case reflect.TypeOf((*ast.StarExpr)(nil)).Elem():
							switch reflect.TypeOf(f.Type.(*ast.StarExpr).X).Elem() {
							case identType:
								elementType = f.Type.(*ast.StarExpr).X.(*ast.Ident).Name
							case reflect.TypeOf((*ast.InterfaceType)(nil)).Elem():
								elementType = "object"
							case reflect.TypeOf((*ast.ArrayType)(nil)).Elem():
								elementType = "array"
							case reflect.TypeOf((*ast.SelectorExpr)(nil)).Elem():
								elementType = f.Type.(*ast.StarExpr).X.(*ast.SelectorExpr).Sel.Name
								// elementType = starExpr.X.(*ast.Ident).Name + "." + starExpr.Sel.Name
							default:
								fmt.Println("What is this?")
								spew.Dump(f.Type.(*ast.StarExpr).X)
								panic("Unknown type. Aborting")
							}
						case reflect.TypeOf((*ast.InterfaceType)(nil)).Elem():
							elementType = "object"
						case reflect.TypeOf((*ast.ArrayType)(nil)).Elem():
							elementType = "array"
						default:
							spew.Dump(typeType)
							panic("Unsupported data type")
						}

						if len(elementType) != 0 {
							var dataType string
							dataType = elementType
							switch elementType {
							case "int8":
								fallthrough
							case "int16":
								fallthrough
							case "int32":
								fallthrough
							case "int64":
								fallthrough
							case "uint":
								fallthrough
							case "uint8":
								fallthrough
							case "uint16":
								fallthrough
							case "uint32":
								fallthrough
							case "uint64":
								fallthrough
							case "int":
								dataType = "integer"
							case "bool":
								dataType = "boolean"
							case "float32":
								fallthrough
							case "float64":
								dataType = "number"
							case "string":
							case "object":
							case "array":
							default:
								dataType = "#/definitions/" + elementType
								// panic("Unknown type! " + elementType)
							}
							if strings.Count(dataType, "#") > 0 {
								jsonSchema.Properties[typeInfo[0]] = SchemaProperty{Ref: dataType}
							} else {
								jsonSchema.Properties[typeInfo[0]] = SchemaProperty{Type: dataType}
							}
							// If the element is not optional, add it to the required slice
							if strings.Count(matches[1], "omitempty") == 0 {
								jsonSchema.Required = append(jsonSchema.Required, matches[1])
							}
						}
					}
				}
			}
			var jsonData []byte
			// jsonData, err := json.Marshal(jsonSchema)
			jsonData, err := json.MarshalIndent(jsonSchema, "", "  ")
			if err != nil {
				log.Println(err)
			}
			fmt.Println(string(jsonData))
			fmt.Println()
		}
	}
}
