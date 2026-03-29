package dynago

import "testing"

type userEntity struct {
	PK   string `dynamo:"PK"`
	SK   string `dynamo:"SK"`
	Name string `dynamo:"Name"`
}

func (u userEntity) DynagoEntity() EntityInfo {
	return EntityInfo{Discriminator: "USER"}
}

type orderEntity struct {
	PK     string `dynamo:"PK"`
	SK     string `dynamo:"SK"`
	Amount int    `dynamo:"Amount"`
}

func (o orderEntity) DynagoEntity() EntityInfo {
	return EntityInfo{Discriminator: "ORDER"}
}

func TestEntity_ReturnsCorrectInfo(t *testing.T) {
	var e Entity = userEntity{Name: "Alice"}
	info := e.DynagoEntity()
	if info.Discriminator != "USER" {
		t.Fatalf("expected discriminator USER, got %q", info.Discriminator)
	}
}

func TestEntity_DifferentTypes(t *testing.T) {
	tests := []struct {
		name   string
		entity Entity
		want   string
	}{
		{"user", userEntity{}, "USER"},
		{"order", orderEntity{}, "ORDER"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := tt.entity.DynagoEntity()
			if info.Discriminator != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, info.Discriminator)
			}
		})
	}
}
