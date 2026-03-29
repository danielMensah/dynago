package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/danielmensah/dynago"
)

// toAWSAttributeValue converts a dynago.AttributeValue to an AWS SDK types.AttributeValue.
func toAWSAttributeValue(av dynago.AttributeValue) types.AttributeValue {
	switch av.Type {
	case dynago.TypeS:
		return &types.AttributeValueMemberS{Value: av.S}
	case dynago.TypeN:
		return &types.AttributeValueMemberN{Value: av.N}
	case dynago.TypeB:
		return &types.AttributeValueMemberB{Value: av.B}
	case dynago.TypeBOOL:
		return &types.AttributeValueMemberBOOL{Value: av.BOOL}
	case dynago.TypeNULL:
		return &types.AttributeValueMemberNULL{Value: av.NULL}
	case dynago.TypeL:
		list := make([]types.AttributeValue, len(av.L))
		for i, v := range av.L {
			list[i] = toAWSAttributeValue(v)
		}
		return &types.AttributeValueMemberL{Value: list}
	case dynago.TypeM:
		m := make(map[string]types.AttributeValue, len(av.M))
		for k, v := range av.M {
			m[k] = toAWSAttributeValue(v)
		}
		return &types.AttributeValueMemberM{Value: m}
	case dynago.TypeSS:
		return &types.AttributeValueMemberSS{Value: av.SS}
	case dynago.TypeNS:
		return &types.AttributeValueMemberNS{Value: av.NS}
	case dynago.TypeBS:
		return &types.AttributeValueMemberBS{Value: av.BS}
	default:
		// Unknown type; return NULL
		return &types.AttributeValueMemberNULL{Value: true}
	}
}

// fromAWSAttributeValue converts an AWS SDK types.AttributeValue to a dynago.AttributeValue.
func fromAWSAttributeValue(av types.AttributeValue) dynago.AttributeValue {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return dynago.AttributeValue{Type: dynago.TypeS, S: v.Value}
	case *types.AttributeValueMemberN:
		return dynago.AttributeValue{Type: dynago.TypeN, N: v.Value}
	case *types.AttributeValueMemberB:
		return dynago.AttributeValue{Type: dynago.TypeB, B: v.Value}
	case *types.AttributeValueMemberBOOL:
		return dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: v.Value}
	case *types.AttributeValueMemberNULL:
		return dynago.AttributeValue{Type: dynago.TypeNULL, NULL: v.Value}
	case *types.AttributeValueMemberL:
		list := make([]dynago.AttributeValue, len(v.Value))
		for i, elem := range v.Value {
			list[i] = fromAWSAttributeValue(elem)
		}
		return dynago.AttributeValue{Type: dynago.TypeL, L: list}
	case *types.AttributeValueMemberM:
		m := make(map[string]dynago.AttributeValue, len(v.Value))
		for k, elem := range v.Value {
			m[k] = fromAWSAttributeValue(elem)
		}
		return dynago.AttributeValue{Type: dynago.TypeM, M: m}
	case *types.AttributeValueMemberSS:
		return dynago.AttributeValue{Type: dynago.TypeSS, SS: v.Value}
	case *types.AttributeValueMemberNS:
		return dynago.AttributeValue{Type: dynago.TypeNS, NS: v.Value}
	case *types.AttributeValueMemberBS:
		return dynago.AttributeValue{Type: dynago.TypeBS, BS: v.Value}
	default:
		return dynago.AttributeValue{Type: dynago.TypeNULL, NULL: true}
	}
}

// toAWSItem converts a dynago item map to an AWS SDK item map.
func toAWSItem(item map[string]dynago.AttributeValue) map[string]types.AttributeValue {
	if item == nil {
		return nil
	}
	out := make(map[string]types.AttributeValue, len(item))
	for k, v := range item {
		out[k] = toAWSAttributeValue(v)
	}
	return out
}

// fromAWSItem converts an AWS SDK item map to a dynago item map.
func fromAWSItem(item map[string]types.AttributeValue) map[string]dynago.AttributeValue {
	if item == nil {
		return nil
	}
	out := make(map[string]dynago.AttributeValue, len(item))
	for k, v := range item {
		out[k] = fromAWSAttributeValue(v)
	}
	return out
}

// toAWSKey is an alias for toAWSItem since keys and items share the same structure.
var toAWSKey = toAWSItem
