package protodump

import (
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ProtoDefinition struct {
	builder     strings.Builder
	indendation int
	pb          *descriptorpb.FileDescriptorProto
	descriptor  protoreflect.FileDescriptor
	filename    string
}

func (pd *ProtoDefinition) indent() {
	pd.indendation += 1
}

func (pd *ProtoDefinition) dedent() {
	pd.indendation -= 1
}

func (pd *ProtoDefinition) writeIndented(s string) {
	pd.builder.WriteString(strings.Repeat("  ", pd.indendation))
	pd.write(s)
}

func (pd *ProtoDefinition) write(s string) {
	pd.builder.WriteString(s)
}

func (pd *ProtoDefinition) String() string {
	return pd.builder.String()
}

func (pd *ProtoDefinition) Filename() string {
	goPackage := pd.pb.GetOptions().GetGoPackage()
	index := strings.Index(goPackage, ";")
	if index == -1 {
		return pd.descriptor.Path()
	}

	return path.Join(goPackage[:index], path.Base(pd.descriptor.Path()))
}

func (pd *ProtoDefinition) writeMethod(method protoreflect.MethodDescriptor) {
	// TODO need to handle method options
	pd.writeIndented("rpc ")
	pd.write(string(method.Name()))
	pd.write(" (")
	if method.IsStreamingClient() {
		pd.write("stream ")
	}
	pd.write(".")
	pd.write(string(method.Input().FullName()))
	pd.write(") returns (")
	if method.IsStreamingServer() {
		pd.write("stream ")
	}
	pd.write(".")
	pd.write(string(method.Output().FullName()))
	pd.write(") {}\n")
}

func (pd *ProtoDefinition) writeService(service protoreflect.ServiceDescriptor) {
	// TODO need to handle service options
	pd.write("service ")
	pd.write(string(service.Name()))
	pd.write(" {\n")
	pd.indent()
	for i := 0; i < service.Methods().Len(); i++ {
		pd.writeMethod(service.Methods().Get(i))
	}
	pd.dedent()
	pd.writeIndented("}\n\n")
}

func (pd *ProtoDefinition) writeType(field protoreflect.FieldDescriptor) {
	kind := field.Kind().String()

	if kind == "message" {
		pd.write(".")
		pd.write(string(field.Message().FullName()))
	} else if kind == "enum" {
		pd.write(".")
		pd.write(string(field.Enum().FullName()))
	} else if kind == "map" {
		pd.write("map<")
		pd.writeType(field.MapKey())
		pd.write(", ")
		pd.writeType(field.MapValue())
		pd.write(">")
	} else {
		pd.write(kind)
	}
}

func (pd *ProtoDefinition) writeOneof(oneof protoreflect.OneofDescriptor) {
	// TODO need to handle oneof options
	if oneof.IsSynthetic() {
		pd.writeField(oneof.Fields().Get(0))
	} else {
		pd.writeIndented("")
		pd.write("oneof ")
		pd.write(string(oneof.Name()))
		pd.write(" {\n")
		pd.indent()
		for i := 0; i < oneof.Fields().Len(); i++ {
			pd.writeField(oneof.Fields().Get(i),1)
		}
		pd.dedent()
		pd.writeIndented("}\n")
	}
}

func (pd *ProtoDefinition) writeField(field protoreflect.FieldDescriptor, is_oneof ...int) {
	// TODO need to handle options
        pd.writeIndented("")
        if len(is_oneof) < 1 {
                if field.HasOptionalKeyword() {
                        pd.write("optional ")
                } else if field.Cardinality().String() == "repeated" {
                        pd.write("repeated ")
                } else if field.Cardinality().String() == "required" && pd.descriptor.Syntax().String() == "proto2" {
                        pd.write("required ")
                }
        }
	pd.writeType(field)
	pd.write(" ")
	pd.write(string(field.Name()))
	pd.write(" = ")
	pd.write(strconv.Itoa(int(field.Number())))
	if field.HasDefault() {
		pd.write(" [default = ")
		kind := field.Kind().String()
		if kind == "string" {
			pd.write(fmt.Sprintf("\"%s\"", field.Default().String()))
		} else if kind == "enum" {
			pd.write(string(field.DefaultEnumValue().Name()))
		} else {
			pd.write(field.Default().String())
		}

                if !field.HasJSONName() {
                        pd.write("]")
                }
        }
        if field.HasJSONName() {
                if !field.HasDefault() {
                        pd.write(" [")
                } else {
                        pd.write(", ")
                }
                pd.write(fmt.Sprintf("json_name=\"%s\"", field.JSONName()))
		pd.write("]")
	}
	pd.write(";\n")
}

func (pd *ProtoDefinition) writeEnum(enum protoreflect.EnumDescriptor) {
	pd.writeIndented("enum ")
	pd.write(string(enum.Name()))
	pd.write(" {\n")
	// TODO need to handle enum options (allow_alias)
	pd.indent()
	for i := 0; i < enum.Values().Len(); i++ {
		value := enum.Values().Get(i)
		pd.writeIndented(string(value.Name()))
		pd.write(" = ")
		pd.write(fmt.Sprintf("%d", value.Number()))
		pd.write(";\n")
	}
	pd.dedent()
	pd.writeIndented("}\n\n")
}

func (pd *ProtoDefinition) writeMessage(message protoreflect.MessageDescriptor) {
	// TODO need to handle message options
	pd.writeIndented("message ")
	pd.write(string(message.Name()))
	pd.write(" {\n")
	pd.indent()

	for i := 0; i < message.ReservedNames().Len(); i++ {
		name := message.ReservedNames().Get(i)
		pd.writeIndented("reserved \"")
		pd.write(string(name))
		pd.write("\";\n")
	}

	for i := 0; i < message.ReservedRanges().Len(); i++ {
		pd.writeIndented("reserved ")
		reservedRange := message.ReservedRanges().Get(i)
		if reservedRange[0] > reservedRange[1] {
			reservedRange[1], reservedRange[0] = reservedRange[0], reservedRange[1]
		}
		reservedRange[1] -= 1
		if reservedRange[0] == reservedRange[1] {
			pd.write(fmt.Sprintf("%d", reservedRange[0]))
		} else {
			pd.write(fmt.Sprintf("%d", reservedRange[0]))
			pd.write(" to ")
			if reservedRange[1] == protowire.MaxValidNumber {
				pd.write("max")
			} else {
				pd.write(fmt.Sprintf("%d", reservedRange[1]))
			}
		}
		pd.write(";\n")
	}

	for i := 0; i < message.Messages().Len(); i++ {
		pd.writeMessage(message.Messages().Get(i))
	}

	for i := 0; i < message.Enums().Len(); i++ {
		pd.writeEnum(message.Enums().Get(i))
	}

	for i := 0; i < message.Fields().Len(); i++ {
		field := message.Fields().Get(i)
		if field.ContainingOneof() == nil {
			pd.writeField(field)
		}
	}

	for i := 0; i < message.Oneofs().Len(); i++ {
		pd.writeOneof(message.Oneofs().Get(i))
	}
	pd.dedent()
	pd.writeIndented("}\n\n")
}

func (pd *ProtoDefinition) writeImport(fileImport protoreflect.FileImport) {
	pd.write("import ")
	if fileImport.IsPublic {
		pd.write("public ")
	}
	pd.write("\"")
	pd.write(fileImport.Path())
	pd.write("\";\n")
}

func (pd *ProtoDefinition) writeStringFileOptions(name string, value string) {
	pd.write("option ")
	pd.write(name)
	pd.write(" = \"")
	pd.write(strings.ReplaceAll(value, "\\", "\\\\"))
	pd.write("\";\n")
}

func (pd *ProtoDefinition) writeBoolFileOptions(name string, value bool) {
	pd.write("option ")
	pd.write(name)
	pd.write(" = ")
	pd.write(strconv.FormatBool(value))
	pd.write(";\n")
}

func (pd *ProtoDefinition) writeFileOptions() {
	optionDefinitions := []struct {
		OptionName string
		FieldName  string
	}{
		{"java_package", "JavaPackage"},
		{"java_outer_classname", "JavaOuterClassname"},
		{"java_multiple_files", "JavaMultipleFiles"},
		{"java_string_check_utf8", "JavaStringCheckUtf8"},
		// TODO OptimizeMode: https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/descriptor.proto#L384
		{"go_package", "GoPackage"},
		// TODO generic services: https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/descriptor.proto#L403
		// TODO deprecated: https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/descriptor.proto#L412
		{"cc_enable_arenas", "CcEnableArenas"},
		{"objc_class_prefix", "ObjcClassPrefix"},
		{"csharp_namespace", "CsharpNamespace"},
		{"swift_prefix", "SwiftPrefix"},
		{"php_class_prefix", "PhpClassPrefix"},
		{"php_namespace", "PhpNamespace"},
		{"php_metadata_namespace", "PhpMetadataNamespace"},
		{"ruby_package", "RubyPackage"},
	}

	optionsPtr := reflect.ValueOf(pd.pb.GetOptions())
	if optionsPtr.IsNil() {
		return
	}
	options := optionsPtr.Elem()
	printedOption := false
	for _, option := range optionDefinitions {
		elemPtr := options.FieldByName(option.FieldName)
		if !elemPtr.IsNil() {
			elem := elemPtr.Elem()
			kind := elem.Kind()
			if kind == reflect.String {
				pd.writeStringFileOptions(option.OptionName, elem.String())
			} else if kind == reflect.Bool {
				pd.writeBoolFileOptions(option.OptionName, elem.Bool())
			}
			printedOption = true
		}
	}

	if printedOption {
		pd.write("\n")
	}
}

func (pd *ProtoDefinition) writeFileDescriptor() {
	pd.write("syntax = \"")
	pd.write(pd.descriptor.Syntax().String())
	pd.write("\";\n\n")

	packageName := pd.descriptor.FullName()
	if packageName != "" {
		pd.write("package ")
		pd.write(string(packageName))
		pd.write(";\n\n")
	}

	pd.writeFileOptions()

	for i := 0; i < pd.descriptor.Imports().Len(); i++ {
		pd.writeImport(pd.descriptor.Imports().Get(i))
	}

	if pd.descriptor.Imports().Len() > 0 {
		pd.write("\n")
	}

	for i := 0; i < pd.descriptor.Services().Len(); i++ {
		pd.writeService(pd.descriptor.Services().Get(i))
	}

	for i := 0; i < pd.descriptor.Messages().Len(); i++ {
		pd.writeMessage(pd.descriptor.Messages().Get(i))
	}

	for i := 0; i < pd.descriptor.Enums().Len(); i++ {
		pd.writeEnum(pd.descriptor.Enums().Get(i))
	}
}

func NewFromBytes(payload []byte) (*ProtoDefinition, error) {
	var pb descriptorpb.FileDescriptorProto
	err := proto.Unmarshal(payload, &pb)
	if err != nil {
		return nil, fmt.Errorf("Couldn't unmarshal proto: %w", err)
	}

	return NewFromDescriptor(&pb)
}

func NewFromDescriptor(pb *descriptorpb.FileDescriptorProto) (*ProtoDefinition, error) {
	fileOptions := protodesc.FileOptions{AllowUnresolvable: true}
	descriptor, err := fileOptions.New(pb, &protoregistry.Files{})

	if err != nil {
		return nil, fmt.Errorf("Couldn't create FileDescriptor: %w", err)
	}

	pd := ProtoDefinition{
		pb:         pb,
		descriptor: descriptor,
	}

	pd.writeFileDescriptor()

	return &pd, nil

}
