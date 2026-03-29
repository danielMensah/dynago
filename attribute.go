package dynago

// AttributeType identifies which variant of AttributeValue is populated.
type AttributeType int

const (
	TypeS    AttributeType = iota + 1 // String
	TypeN                             // Number (stored as string)
	TypeB                             // Binary
	TypeBOOL                          // Boolean
	TypeNULL                          // Null
	TypeL                             // List
	TypeM                             // Map
	TypeSS                            // String Set
	TypeNS                            // Number Set
	TypeBS                            // Binary Set
)

// AttributeValue is a library-owned sum type representing a DynamoDB
// attribute value. Exactly one typed field is populated, identified by Type.
type AttributeValue struct {
	Type AttributeType

	S    string
	N    string // DynamoDB numbers are transmitted as strings
	B    []byte
	BOOL bool
	NULL bool
	L    []AttributeValue
	M    map[string]AttributeValue
	SS   []string
	NS   []string
	BS   [][]byte
}
