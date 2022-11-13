package ast

type Node interface {
	GetKind() string
	GetLoc() *Location
}

// The list of all possible AST node graphql.
// Ensure that all node types implements Node interface
var _ Node = (*Name)(nil)

var (
	_ Node = (*Document)(nil)
	_ Node = (*OperationDefinition)(nil)
	_ Node = (*VariableDefinition)(nil)
	_ Node = (*Variable)(nil)
	_ Node = (*SelectionSet)(nil)
	_ Node = (*Field)(nil)
	_ Node = (*Argument)(nil)
	_ Node = (*FragmentSpread)(nil)
	_ Node = (*InlineFragment)(nil)
	_ Node = (*FragmentDefinition)(nil)
	_ Node = (*IntValue)(nil)
	_ Node = (*FloatValue)(nil)
	_ Node = (*StringValue)(nil)
	_ Node = (*BooleanValue)(nil)
	_ Node = (*EnumValue)(nil)
	_ Node = (*ListValue)(nil)
	_ Node = (*ObjectValue)(nil)
	_ Node = (*ObjectField)(nil)
	_ Node = (*Directive)(nil)
	_ Node = (*Named)(nil)
	_ Node = (*List)(nil)
	_ Node = (*NonNull)(nil)
	_ Node = (*SchemaDefinition)(nil)
	_ Node = (*OperationTypeDefinition)(nil)
	_ Node = (*ScalarDefinition)(nil)
	_ Node = (*ObjectDefinition)(nil)
	_ Node = (*FieldDefinition)(nil)
	_ Node = (*InputValueDefinition)(nil)
	_ Node = (*InterfaceDefinition)(nil)
	_ Node = (*UnionDefinition)(nil)
	_ Node = (*EnumDefinition)(nil)
	_ Node = (*EnumValueDefinition)(nil)
	_ Node = (*InputObjectDefinition)(nil)
	_ Node = (*TypeExtensionDefinition)(nil)
	_ Node = (*DirectiveDefinition)(nil)
)
