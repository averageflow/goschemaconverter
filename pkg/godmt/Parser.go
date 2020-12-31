package godmt

import (
	"go/ast"
	"reflect"
)

func ParseStruct(d *ast.Ident) []ScannedStruct {
	var result []ScannedStruct

	structTypes := reflect.ValueOf(d.Obj.Decl).Elem().FieldByName("Type")
	if !structTypes.IsValid() {
		return result
	}

	switch structTypes.Interface().(type) {
	case *ast.StructType:
		fields := structTypes.Interface().(*ast.StructType)
		fieldList := fields.Fields.List

		var rawScannedFields []ScannedStructField

		for i := range fieldList {
			parsed := ParseStructField(fieldList[i])
			if parsed != nil {
				rawScannedFields = append(rawScannedFields, *parsed)
			}
		}

		result = append(result, ScannedStruct{
			Doc:          nil,
			Name:         d.Name,
			Fields:       rawScannedFields,
			InternalType: StructType,
		})

	default:
		break
	}

	return result
}

func ParseStructField(item *ast.Field) *ScannedStructField {
	switch item.Type.(type) { //nolint:gocritic
	case *ast.Ident:
		return SimpleStructFieldParser(item)
	case *ast.StructType:
		return ParseNestedStruct(item)
	case *ast.SelectorExpr:
		return ImportedStructFieldParser(item)
	case *ast.MapType:
		return SimpleStructFieldParser(item)
	case *ast.ArrayType:
		return ParseComplexStructField(item.Names[0])
	case *ast.StarExpr:
		pointer := item.Type.(*ast.StarExpr).X
		switch value := pointer.(type) {
		// case *ast.ArrayType: // FUTURE: do array and map pointers
		case *ast.Ident:
			return &ScannedStructField{
				Doc:          ExtractComments(item.Doc),
				Name:         item.Names[0].Name,
				Kind:         value.Name,
				Tag:          item.Tag.Value,
				InternalType: VarType,
				IsPointer:    true,
			}

		default:
			return nil
		}
	default:
		return nil
	}
}

func ParseNestedStruct(field *ast.Field) *ScannedStructField {
	nestedFields := reflect.ValueOf(field.Type).Elem().FieldByName("Fields").Interface().(*ast.FieldList)

	var parsedNestedFields []ScannedStructField

	for i := range nestedFields.List {
		parsedField := ParseStructField(nestedFields.List[i])
		if parsedField != nil {
			parsedNestedFields = append(parsedNestedFields, *parsedField)
		}
	}

	tag := field.Tag

	var tagValue string
	if tag != nil {
		tagValue = tag.Value
	}

	return &ScannedStructField{
		Name:          field.Names[0].Name,
		Kind:          "struct",
		Tag:           tagValue,
		Doc:           ExtractComments(field.Doc),
		ImportDetails: nil,
		SubFields:     parsedNestedFields,
	}
}

func ParseComplexStructField(item *ast.Ident) *ScannedStructField {
	decl := item.Obj.Decl
	tag := reflect.ValueOf(decl).Elem().FieldByName("Tag").Interface().(*ast.BasicLit)
	comments := reflect.ValueOf(decl).Elem().FieldByName("Doc").Interface().(*ast.CommentGroup)

	objectType := reflect.ValueOf(decl).Elem().FieldByName("Type").Interface()

	var kind string

	var internalType int

	switch objectTypeDetails := objectType.(type) {
	case *ast.ArrayType:
		internalType = SliceType
		kind = GetSliceType(objectTypeDetails)
	default:
		return nil
	}

	return &ScannedStructField{
		Name:          item.Name,
		Kind:          kind,
		Tag:           tag.Value,
		Doc:           ExtractComments(comments),
		ImportDetails: nil,
		InternalType:  internalType,
	}
}

func ParseConstantsAndVariables(d *ast.Ident) []ScannedType {
	var result []ScannedType

	objectValues := reflect.ValueOf(d.Obj.Decl).Elem().FieldByName("Values")
	if !objectValues.IsValid() {
		return result
	}

	values := objectValues.Interface().([]ast.Expr)

	for i := range values {
		switch item := values[i].(type) {
		case *ast.BasicLit:
			parsed := BasicTypeLiteralParser(d, item)
			result = append(result, parsed)

		case *ast.Ident:
			parsed := IdentifierParser(d, item)

			if parsed != nil {
				result = append(result, *parsed)
			}

		case *ast.CompositeLit:
			switch itemType := item.Type.(type) {
			case *ast.MapType:
				mapElements := reflect.ValueOf(item.Elts).Interface().([]ast.Expr)
				parsed := CompositeLiteralMapParser(d, mapElements, item)
				result = append(result, parsed)
			case *ast.ArrayType:
				sliceType := GetMapValueType(itemType.Elt)
				parsed := CompositeLiteralSliceParser(d, sliceType, item)
				result = append(result, parsed)
			}
		}
	}

	return result
}
