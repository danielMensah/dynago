package expr

import "testing"

func BenchmarkExprTranslate_ToDynamo(b *testing.B) {
	b.ReportAllocs()
	// Pre-build the AST.
	ast, err := ParseCondition("Active = ? AND Age > ? AND Status <> ? AND contains(Name, ?)", true, 21, "inactive", "test")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		ToDynamo(ast)
	}
}
