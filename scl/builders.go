package scl

func NewLogicalNodeType(id, lnClass string, dataObjects ...*DataObjectDefinition) *LogicalNodeType {
	return &LogicalNodeType{
		sclType:               sclType{Id: id},
		LnClass:               lnClass,
		DataObjectDefinitions: dataObjects,
	}
}

func NewDataObjectType(id, cdc string, dataAttributes ...*DataAttributeDefinition) *DataObjectType {
	return &DataObjectType{
		sclType:        sclType{Id: id},
		Cdc:            cdc,
		DataAttributes: dataAttributes,
	}
}

func NewDataAttributeType(id string, subDataAttributes ...*DataAttributeDefinition) *DataAttributeType {
	return &DataAttributeType{
		sclType:           sclType{Id: id},
		SubDataAttributes: subDataAttributes,
	}
}

func NewEnumerationType(id string, enumValues ...*EnumerationValue) *EnumerationType {
	return &EnumerationType{
		sclType:    sclType{Id: id},
		EnumValues: enumValues,
	}
}
