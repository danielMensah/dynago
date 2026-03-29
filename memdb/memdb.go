// Package memdb provides an in-memory implementation of the dynago.Backend
// interface for testing. Tables must be created with CreateTable before use.
package memdb

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/danielmensah/dynago"
)

// KeyType identifies the DynamoDB key attribute type.
type KeyType int

const (
	// StringKey indicates the key attribute is a string (S).
	StringKey KeyType = iota + 1
	// NumberKey indicates the key attribute is a number (N).
	NumberKey
	// BinaryKey indicates the key attribute is binary (B).
	BinaryKey
)

// KeyDef defines a key attribute (name + type).
type KeyDef struct {
	Name string
	Type KeyType
}

// GSISchema defines the schema of a Global Secondary Index.
type GSISchema struct {
	Name     string
	HashKey  KeyDef
	RangeKey *KeyDef // optional
}

// TableSchema defines the key schema and optional GSIs for a table.
type TableSchema struct {
	HashKey  KeyDef
	RangeKey *KeyDef // optional
	GSIs     []GSISchema
}

// tableData holds the storage and schema for a single table.
type tableData struct {
	schema TableSchema
	mu     sync.RWMutex
	items  map[string]map[string]map[string]dynago.AttributeValue // hash -> range -> item
	gsis   map[string]*gsiData
}

// gsiData holds the storage for a single GSI.
type gsiData struct {
	schema GSISchema
	items  map[string]map[string]map[string]dynago.AttributeValue // gsi_hash -> gsi_range -> item
}

// MemoryBackend is an in-memory Backend for testing.
type MemoryBackend struct {
	mu     sync.RWMutex
	tables map[string]*tableData
}

// New creates a new empty MemoryBackend.
func New() *MemoryBackend {
	return &MemoryBackend{tables: make(map[string]*tableData)}
}

// CreateTable registers a table with the given schema. It panics if the table already exists.
func (m *MemoryBackend) CreateTable(name string, schema TableSchema) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.tables[name]; exists {
		panic(fmt.Sprintf("memdb: table %q already exists", name))
	}
	td := &tableData{
		schema: schema,
		items:  make(map[string]map[string]map[string]dynago.AttributeValue),
		gsis:   make(map[string]*gsiData),
	}
	for _, gsi := range schema.GSIs {
		td.gsis[gsi.Name] = &gsiData{
			schema: gsi,
			items:  make(map[string]map[string]map[string]dynago.AttributeValue),
		}
	}
	m.tables[name] = td
}

// table returns the tableData for the given table name, or an error if it doesn't exist.
func (m *MemoryBackend) table(name string) (*tableData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	td, ok := m.tables[name]
	if !ok {
		return nil, &dynago.Error{
			Sentinel: dynago.ErrValidation,
			Message:  fmt.Sprintf("memdb: table %q does not exist", name),
		}
	}
	return td, nil
}

// keyString converts an AttributeValue to a string suitable for use as a map key.
func keyString(av dynago.AttributeValue) string {
	switch av.Type {
	case dynago.TypeS:
		return "S:" + av.S
	case dynago.TypeN:
		return "N:" + av.N
	case dynago.TypeB:
		return "B:" + string(av.B)
	default:
		return ""
	}
}

// extractKey extracts the hash and range key strings from an item using the table schema.
func (td *tableData) extractKey(item map[string]dynago.AttributeValue) (hash string, rangeKey string, err error) {
	hv, ok := item[td.schema.HashKey.Name]
	if !ok {
		return "", "", &dynago.Error{
			Sentinel: dynago.ErrValidation,
			Message:  fmt.Sprintf("memdb: missing hash key %q", td.schema.HashKey.Name),
		}
	}
	hash = keyString(hv)
	if hash == "" {
		return "", "", &dynago.Error{
			Sentinel: dynago.ErrValidation,
			Message:  fmt.Sprintf("memdb: invalid type for hash key %q", td.schema.HashKey.Name),
		}
	}

	if td.schema.RangeKey != nil {
		rv, ok := item[td.schema.RangeKey.Name]
		if !ok {
			return "", "", &dynago.Error{
				Sentinel: dynago.ErrValidation,
				Message:  fmt.Sprintf("memdb: missing range key %q", td.schema.RangeKey.Name),
			}
		}
		rangeKey = keyString(rv)
		if rangeKey == "" {
			return "", "", &dynago.Error{
				Sentinel: dynago.ErrValidation,
				Message:  fmt.Sprintf("memdb: invalid type for range key %q", td.schema.RangeKey.Name),
			}
		}
	} else {
		rangeKey = "_"
	}

	return hash, rangeKey, nil
}

// extractKeyFromMap extracts key strings using explicit key definitions (for GSIs).
func extractKeyFromDef(item map[string]dynago.AttributeValue, hashDef KeyDef, rangeDef *KeyDef) (hash string, rangeKey string, ok bool) {
	hv, exists := item[hashDef.Name]
	if !exists {
		return "", "", false
	}
	hash = keyString(hv)
	if hash == "" {
		return "", "", false
	}
	if rangeDef != nil {
		rv, exists := item[rangeDef.Name]
		if !exists {
			return "", "", false
		}
		rangeKey = keyString(rv)
		if rangeKey == "" {
			return "", "", false
		}
	} else {
		rangeKey = "_"
	}
	return hash, rangeKey, true
}

// deepCopyItem creates a deep copy of a DynamoDB item.
func deepCopyItem(item map[string]dynago.AttributeValue) map[string]dynago.AttributeValue {
	if item == nil {
		return nil
	}
	out := make(map[string]dynago.AttributeValue, len(item))
	for k, v := range item {
		out[k] = deepCopyAV(v)
	}
	return out
}

// deepCopyAV creates a deep copy of an AttributeValue.
func deepCopyAV(av dynago.AttributeValue) dynago.AttributeValue {
	cp := av
	switch av.Type {
	case dynago.TypeB:
		if av.B != nil {
			cp.B = make([]byte, len(av.B))
			copy(cp.B, av.B)
		}
	case dynago.TypeL:
		if av.L != nil {
			cp.L = make([]dynago.AttributeValue, len(av.L))
			for i, v := range av.L {
				cp.L[i] = deepCopyAV(v)
			}
		}
	case dynago.TypeM:
		if av.M != nil {
			cp.M = make(map[string]dynago.AttributeValue, len(av.M))
			for k, v := range av.M {
				cp.M[k] = deepCopyAV(v)
			}
		}
	case dynago.TypeSS:
		if av.SS != nil {
			cp.SS = make([]string, len(av.SS))
			copy(cp.SS, av.SS)
		}
	case dynago.TypeNS:
		if av.NS != nil {
			cp.NS = make([]string, len(av.NS))
			copy(cp.NS, av.NS)
		}
	case dynago.TypeBS:
		if av.BS != nil {
			cp.BS = make([][]byte, len(av.BS))
			for i, b := range av.BS {
				cp.BS[i] = make([]byte, len(b))
				copy(cp.BS[i], b)
			}
		}
	}
	return cp
}

// storeItem stores a deep copy of the item into the table's storage.
func (td *tableData) storeItem(hash, rangeKey string, item map[string]dynago.AttributeValue) {
	if _, ok := td.items[hash]; !ok {
		td.items[hash] = make(map[string]map[string]dynago.AttributeValue)
	}
	td.items[hash][rangeKey] = deepCopyItem(item)
}

// getItem retrieves an item from the table, returning nil if not found.
func (td *tableData) getItem(hash, rangeKey string) map[string]dynago.AttributeValue {
	if rng, ok := td.items[hash]; ok {
		if item, ok := rng[rangeKey]; ok {
			return item
		}
	}
	return nil
}

// deleteItem removes an item from the table, returning the old item or nil.
func (td *tableData) deleteItem(hash, rangeKey string) map[string]dynago.AttributeValue {
	if rng, ok := td.items[hash]; ok {
		if item, ok := rng[rangeKey]; ok {
			delete(rng, rangeKey)
			if len(rng) == 0 {
				delete(td.items, hash)
			}
			return item
		}
	}
	return nil
}

// updateGSIs updates all GSI indexes for an item change.
// oldItem is the previous item (nil if new), newItem is the new item (nil if deleted).
func (td *tableData) updateGSIs(oldItem, newItem map[string]dynago.AttributeValue) {
	for _, gsi := range td.gsis {
		// Remove old entry
		if oldItem != nil {
			if hash, rng, ok := extractKeyFromDef(oldItem, gsi.schema.HashKey, gsi.schema.RangeKey); ok {
				if rngMap, exists := gsi.items[hash]; exists {
					delete(rngMap, rng)
					if len(rngMap) == 0 {
						delete(gsi.items, hash)
					}
				}
			}
		}
		// Add new entry
		if newItem != nil {
			if hash, rng, ok := extractKeyFromDef(newItem, gsi.schema.HashKey, gsi.schema.RangeKey); ok {
				if _, exists := gsi.items[hash]; !exists {
					gsi.items[hash] = make(map[string]map[string]dynago.AttributeValue)
				}
				gsi.items[hash][rng] = deepCopyItem(newItem)
			}
		}
	}
}

// projectItem filters item attributes based on a projection expression.
func projectItem(item map[string]dynago.AttributeValue, projection string, names map[string]string) map[string]dynago.AttributeValue {
	if projection == "" || item == nil {
		return item
	}
	attrs := parseProjectionAttrs(projection, names)
	result := make(map[string]dynago.AttributeValue, len(attrs))
	for _, attr := range attrs {
		if v, ok := item[attr]; ok {
			result[attr] = v
		}
	}
	return result
}

// parseProjectionAttrs extracts attribute names from a projection expression.
func parseProjectionAttrs(projection string, names map[string]string) []string {
	parts := splitCSV(projection)
	var attrs []string
	for _, p := range parts {
		p = trimSpace(p)
		if p == "" {
			continue
		}
		// Resolve #name placeholders
		if len(p) > 0 && p[0] == '#' {
			if resolved, ok := names[p]; ok {
				attrs = append(attrs, resolved)
				continue
			}
		}
		attrs = append(attrs, p)
	}
	return attrs
}

func splitCSV(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

// --- Backend interface stubs ---

func (m *MemoryBackend) GetItem(_ context.Context, req *dynago.GetItemRequest) (*dynago.GetItemResponse, error) {
	td, err := m.table(req.TableName)
	if err != nil {
		return nil, err
	}
	td.mu.RLock()
	defer td.mu.RUnlock()

	hash, rng, err := td.extractKey(req.Key)
	if err != nil {
		return nil, err
	}

	item := td.getItem(hash, rng)
	if item == nil {
		return &dynago.GetItemResponse{}, nil
	}

	result := deepCopyItem(item)
	result = projectItem(result, req.ProjectionExpression, req.ExpressionAttributeNames)
	return &dynago.GetItemResponse{Item: result}, nil
}

func (m *MemoryBackend) PutItem(_ context.Context, req *dynago.PutItemRequest) (*dynago.PutItemResponse, error) {
	td, err := m.table(req.TableName)
	if err != nil {
		return nil, err
	}
	td.mu.Lock()
	defer td.mu.Unlock()

	hash, rng, err := td.extractKey(req.Item)
	if err != nil {
		return nil, err
	}

	// Evaluate condition expression if present
	if req.ConditionExpression != "" {
		existing := td.getItem(hash, rng)
		if existing == nil {
			existing = map[string]dynago.AttributeValue{}
		}
		ok, evalErr := evalCondition(req.ConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, existing)
		if evalErr != nil {
			return nil, evalErr
		}
		if !ok {
			return nil, &dynago.Error{Sentinel: dynago.ErrConditionFailed, Message: "memdb: condition check failed"}
		}
	}

	oldItem := td.getItem(hash, rng)
	td.storeItem(hash, rng, req.Item)
	td.updateGSIs(oldItem, req.Item)

	return &dynago.PutItemResponse{}, nil
}

func (m *MemoryBackend) DeleteItem(_ context.Context, req *dynago.DeleteItemRequest) (*dynago.DeleteItemResponse, error) {
	td, err := m.table(req.TableName)
	if err != nil {
		return nil, err
	}
	td.mu.Lock()
	defer td.mu.Unlock()

	hash, rng, err := td.extractKey(req.Key)
	if err != nil {
		return nil, err
	}

	// Evaluate condition expression if present
	if req.ConditionExpression != "" {
		existing := td.getItem(hash, rng)
		if existing == nil {
			existing = map[string]dynago.AttributeValue{}
		}
		ok, evalErr := evalCondition(req.ConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, existing)
		if evalErr != nil {
			return nil, evalErr
		}
		if !ok {
			return nil, &dynago.Error{Sentinel: dynago.ErrConditionFailed, Message: "memdb: condition check failed"}
		}
	}

	oldItem := td.deleteItem(hash, rng)
	td.updateGSIs(oldItem, nil)

	resp := &dynago.DeleteItemResponse{}
	// Note: ReturnValues ALL_OLD would need to be added to DeleteItemResponse
	_ = req.ReturnValues
	return resp, nil
}

func (m *MemoryBackend) UpdateItem(_ context.Context, req *dynago.UpdateItemRequest) (*dynago.UpdateItemResponse, error) {
	td, err := m.table(req.TableName)
	if err != nil {
		return nil, err
	}
	td.mu.Lock()
	defer td.mu.Unlock()

	hash, rng, err := td.extractKey(req.Key)
	if err != nil {
		return nil, err
	}

	existing := td.getItem(hash, rng)

	// Evaluate condition expression if present
	if req.ConditionExpression != "" {
		checkItem := existing
		if checkItem == nil {
			checkItem = map[string]dynago.AttributeValue{}
		}
		ok, evalErr := evalCondition(req.ConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, checkItem)
		if evalErr != nil {
			return nil, evalErr
		}
		if !ok {
			return nil, &dynago.Error{Sentinel: dynago.ErrConditionFailed, Message: "memdb: condition check failed"}
		}
	}

	// Start with existing or new item with keys
	var item map[string]dynago.AttributeValue
	if existing != nil {
		item = deepCopyItem(existing)
	} else {
		item = deepCopyItem(req.Key) // upsert: start with keys
	}

	oldItem := deepCopyItem(existing)

	// Apply update expression
	if req.UpdateExpression != "" {
		nodes, parseErr := parseUpdateExpression(req.UpdateExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues)
		if parseErr != nil {
			return nil, parseErr
		}
		updated, evalErr := evalUpdateNodes(nodes, item)
		if evalErr != nil {
			return nil, evalErr
		}
		item = updated
	}

	td.storeItem(hash, rng, item)
	td.updateGSIs(oldItem, item)

	resp := &dynago.UpdateItemResponse{}
	switch req.ReturnValues {
	case "ALL_NEW":
		resp.Attributes = deepCopyItem(item)
	case "ALL_OLD":
		if oldItem != nil {
			resp.Attributes = oldItem
		}
	}
	return resp, nil
}

func (m *MemoryBackend) Query(_ context.Context, req *dynago.QueryRequest) (*dynago.QueryResponse, error) {
	td, err := m.table(req.TableName)
	if err != nil {
		return nil, err
	}
	td.mu.RLock()
	defer td.mu.RUnlock()

	// Determine which data source and key schema to use.
	var items map[string]map[string]map[string]dynago.AttributeValue
	var hashKeyDef KeyDef
	var rangeKeyDef *KeyDef

	if req.IndexName != "" {
		gsi, ok := td.gsis[req.IndexName]
		if !ok {
			return nil, &dynago.Error{
				Sentinel: dynago.ErrValidation,
				Message:  fmt.Sprintf("memdb: index %q does not exist on table %q", req.IndexName, req.TableName),
			}
		}
		items = gsi.items
		hashKeyDef = gsi.schema.HashKey
		rangeKeyDef = gsi.schema.RangeKey
	} else {
		items = td.items
		hashKeyDef = td.schema.HashKey
		rangeKeyDef = td.schema.RangeKey
	}

	// Parse the key condition expression to extract partition key value and sort key condition.
	pkValue, skCond, err := parseKeyCondition(req.KeyConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, hashKeyDef.Name, rangeKeyDef)
	if err != nil {
		return nil, err
	}

	// Look up the partition.
	hashKey := keyString(pkValue)
	partition, ok := items[hashKey]
	if !ok {
		return &dynago.QueryResponse{}, nil
	}

	// Collect and sort range keys.
	rangeKeys := make([]string, 0, len(partition))
	for rk := range partition {
		rangeKeys = append(rangeKeys, rk)
	}
	sort.Strings(rangeKeys)

	// Apply sort direction.
	ascending := true
	if req.ScanIndexForward != nil && !*req.ScanIndexForward {
		ascending = false
	}
	if !ascending {
		// Reverse the sorted keys.
		for i, j := 0, len(rangeKeys)-1; i < j; i, j = i+1, j-1 {
			rangeKeys[i], rangeKeys[j] = rangeKeys[j], rangeKeys[i]
		}
	}

	// Find ExclusiveStartKey position.
	startIdx := 0
	if len(req.ExclusiveStartKey) > 0 {
		startRangeKey := "_"
		if rangeKeyDef != nil {
			if rv, exists := req.ExclusiveStartKey[rangeKeyDef.Name]; exists {
				startRangeKey = keyString(rv)
			}
		}
		// Skip past the start key.
		for i, rk := range rangeKeys {
			if rk == startRangeKey {
				startIdx = i + 1
				break
			}
		}
	}

	// Build the table key schema for building LastEvaluatedKey.
	tableHashDef := td.schema.HashKey
	tableRangeDef := td.schema.RangeKey

	// First, collect candidate items matching the sort key condition.
	type candidate struct {
		item map[string]dynago.AttributeValue
	}
	var candidates []candidate
	for i := startIdx; i < len(rangeKeys); i++ {
		rk := rangeKeys[i]
		item := partition[rk]
		if skCond != nil && !matchesSortKeyCondition(item, skCond, rangeKeyDef) {
			continue
		}
		candidates = append(candidates, candidate{item: item})
	}

	// Apply Limit: DynamoDB Limit caps items evaluated (scanned), not results.
	limit := int(req.Limit)
	hasMore := false
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
		hasMore = true
	}

	scannedCount := int32(len(candidates))
	var resultItems []map[string]dynago.AttributeValue

	for _, c := range candidates {
		// Apply filter expression.
		if req.FilterExpression != "" {
			matched, evalErr := evalCondition(req.FilterExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, c.item)
			if evalErr != nil {
				return nil, evalErr
			}
			if !matched {
				continue
			}
		}
		// Apply projection.
		resultItem := deepCopyItem(c.item)
		resultItem = projectItem(resultItem, req.ProjectionExpression, req.ExpressionAttributeNames)
		resultItems = append(resultItems, resultItem)
	}

	resp := &dynago.QueryResponse{
		Items:        resultItems,
		Count:        int32(len(resultItems)),
		ScannedCount: scannedCount,
	}

	// Set LastEvaluatedKey if there are more items.
	if hasMore && len(candidates) > 0 {
		lastItem := candidates[len(candidates)-1].item
		resp.LastEvaluatedKey = buildLastEvaluatedKey(lastItem, tableHashDef, tableRangeDef)
	}

	return resp, nil
}

// buildLastEvaluatedKey constructs the LastEvaluatedKey from an item and table key schema.
func buildLastEvaluatedKey(item map[string]dynago.AttributeValue, hashDef KeyDef, rangeDef *KeyDef) map[string]dynago.AttributeValue {
	key := make(map[string]dynago.AttributeValue)
	if v, ok := item[hashDef.Name]; ok {
		key[hashDef.Name] = deepCopyAV(v)
	}
	if rangeDef != nil {
		if v, ok := item[rangeDef.Name]; ok {
			key[rangeDef.Name] = deepCopyAV(v)
		}
	}
	return key
}

// sortKeyCondition holds a parsed sort key condition.
type sortKeyCondition struct {
	op     string // "=", "<", "<=", ">", ">=", "begins_with", "BETWEEN"
	value  dynago.AttributeValue
	value2 dynago.AttributeValue // used for BETWEEN high bound
	attr   string                // resolved sort key attribute name
}

// parseKeyCondition extracts the partition key value and optional sort key condition
// from a KeyConditionExpression like "#pk = :pk0 AND begins_with(#sk, :sk0)".
func parseKeyCondition(expr string, names map[string]string, values map[string]dynago.AttributeValue, pkName string, rangeDef *KeyDef) (dynago.AttributeValue, *sortKeyCondition, error) {
	// Split on AND (case insensitive).
	parts := splitOnAND(expr)
	if len(parts) == 0 || len(parts) > 2 {
		return dynago.AttributeValue{}, nil, &dynago.Error{
			Sentinel: dynago.ErrValidation,
			Message:  fmt.Sprintf("memdb: invalid key condition expression: %q", expr),
		}
	}

	var pkValue dynago.AttributeValue
	var skCond *sortKeyCondition
	pkFound := false

	for _, part := range parts {
		part = trimSpace(part)

		// Check for begins_with function.
		lowerPart := strings.ToLower(part)
		if strings.HasPrefix(lowerPart, "begins_with(") || strings.HasPrefix(lowerPart, "begins_with (") {
			sc, err := parseBeginsWith(part, names, values, rangeDef)
			if err != nil {
				return dynago.AttributeValue{}, nil, err
			}
			skCond = sc
			continue
		}

		// Check for BETWEEN.
		if containsBETWEEN(part) {
			sc, err := parseBetween(part, names, values, rangeDef)
			if err != nil {
				return dynago.AttributeValue{}, nil, err
			}
			skCond = sc
			continue
		}

		// Parse comparison: attr op value.
		attr, op, val, err := parseSimpleComparison(part, names, values)
		if err != nil {
			return dynago.AttributeValue{}, nil, err
		}

		if attr == pkName && op == "=" {
			pkValue = val
			pkFound = true
		} else if rangeDef != nil && attr == rangeDef.Name {
			skCond = &sortKeyCondition{
				op:    op,
				value: val,
				attr:  attr,
			}
		} else {
			return dynago.AttributeValue{}, nil, &dynago.Error{
				Sentinel: dynago.ErrValidation,
				Message:  fmt.Sprintf("memdb: unexpected attribute %q in key condition", attr),
			}
		}
	}

	if !pkFound {
		return dynago.AttributeValue{}, nil, &dynago.Error{
			Sentinel: dynago.ErrValidation,
			Message:  "memdb: partition key condition not found in key condition expression",
		}
	}

	return pkValue, skCond, nil
}

// splitOnAND splits a key condition expression on the top-level " AND " that
// separates the partition key condition from the sort key condition. It must
// not split on the AND inside "BETWEEN x AND y". It does this by splitting
// into at most 2 parts: the first equality condition for the partition key,
// and everything else as the sort key condition.
func splitOnAND(s string) []string {
	upper := strings.ToUpper(s)
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '(' {
			depth++
		} else if s[i] == ')' {
			depth--
		} else if depth == 0 && i+5 <= len(s) && upper[i:i+5] == " AND " {
			left := trimSpace(s[:i])
			right := trimSpace(s[i+5:])
			// If the left side is a simple equality (contains = but not BETWEEN),
			// this is the partition/sort key split.
			leftUpper := strings.ToUpper(left)
			if !strings.Contains(leftUpper, " BETWEEN ") {
				return []string{left, right}
			}
		}
	}
	return []string{s}
}

// containsBETWEEN checks if the part contains a BETWEEN keyword (case insensitive).
func containsBETWEEN(s string) bool {
	upper := strings.ToUpper(s)
	return strings.Contains(upper, " BETWEEN ")
}

// parseBeginsWith parses "begins_with(#sk, :sk0)" style expressions.
func parseBeginsWith(part string, names map[string]string, values map[string]dynago.AttributeValue, rangeDef *KeyDef) (*sortKeyCondition, error) {
	// Extract content inside parentheses.
	openParen := strings.Index(part, "(")
	closeParen := strings.LastIndex(part, ")")
	if openParen < 0 || closeParen < 0 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: invalid begins_with expression"}
	}
	inner := part[openParen+1 : closeParen]
	args := strings.SplitN(inner, ",", 2)
	if len(args) != 2 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: begins_with requires 2 arguments"}
	}

	attrTok := trimSpace(args[0])
	valTok := trimSpace(args[1])

	attrName := resolveNamePlaceholder(attrTok, names)
	val, err := resolveValuePlaceholder(valTok, values)
	if err != nil {
		return nil, err
	}

	return &sortKeyCondition{
		op:    "begins_with",
		value: val,
		attr:  attrName,
	}, nil
}

// parseBetween parses "#sk BETWEEN :lo AND :hi" style expressions.
func parseBetween(part string, names map[string]string, values map[string]dynago.AttributeValue, rangeDef *KeyDef) (*sortKeyCondition, error) {
	upper := strings.ToUpper(part)
	betweenIdx := strings.Index(upper, " BETWEEN ")
	if betweenIdx < 0 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: invalid BETWEEN expression"}
	}
	attrTok := trimSpace(part[:betweenIdx])
	rest := part[betweenIdx+9:] // after " BETWEEN "

	// Split rest on AND.
	andUpper := strings.ToUpper(rest)
	andIdx := strings.Index(andUpper, " AND ")
	if andIdx < 0 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: BETWEEN requires AND"}
	}
	loTok := trimSpace(rest[:andIdx])
	hiTok := trimSpace(rest[andIdx+5:])

	attrName := resolveNamePlaceholder(attrTok, names)
	loVal, err := resolveValuePlaceholder(loTok, values)
	if err != nil {
		return nil, err
	}
	hiVal, err := resolveValuePlaceholder(hiTok, values)
	if err != nil {
		return nil, err
	}

	return &sortKeyCondition{
		op:     "BETWEEN",
		value:  loVal,
		value2: hiVal,
		attr:   attrName,
	}, nil
}

// parseSimpleComparison parses "attr op value" from a string like "#pk = :pk0".
func parseSimpleComparison(part string, names map[string]string, values map[string]dynago.AttributeValue) (string, string, dynago.AttributeValue, error) {
	// Find the operator.
	ops := []string{"<=", ">=", "<>", "=", "<", ">"}
	for _, op := range ops {
		idx := strings.Index(part, op)
		if idx >= 0 {
			attrTok := trimSpace(part[:idx])
			valTok := trimSpace(part[idx+len(op):])
			attrName := resolveNamePlaceholder(attrTok, names)
			val, err := resolveValuePlaceholder(valTok, values)
			if err != nil {
				return "", "", dynago.AttributeValue{}, err
			}
			return attrName, op, val, nil
		}
	}
	return "", "", dynago.AttributeValue{}, &dynago.Error{
		Sentinel: dynago.ErrValidation,
		Message:  fmt.Sprintf("memdb: cannot parse key condition part: %q", part),
	}
}

// resolveNamePlaceholder resolves #name placeholders to attribute names.
func resolveNamePlaceholder(tok string, names map[string]string) string {
	if len(tok) > 0 && tok[0] == '#' {
		if resolved, ok := names[tok]; ok {
			return resolved
		}
	}
	return tok
}

// resolveValuePlaceholder resolves :value placeholders to AttributeValues.
func resolveValuePlaceholder(tok string, values map[string]dynago.AttributeValue) (dynago.AttributeValue, error) {
	if len(tok) > 0 && tok[0] == ':' {
		if v, ok := values[tok]; ok {
			return v, nil
		}
		return dynago.AttributeValue{}, &dynago.Error{
			Sentinel: dynago.ErrValidation,
			Message:  fmt.Sprintf("memdb: unknown value placeholder %q", tok),
		}
	}
	return dynago.AttributeValue{}, &dynago.Error{
		Sentinel: dynago.ErrValidation,
		Message:  fmt.Sprintf("memdb: expected value placeholder, got %q", tok),
	}
}

// matchesSortKeyCondition checks if an item matches the sort key condition.
func matchesSortKeyCondition(item map[string]dynago.AttributeValue, cond *sortKeyCondition, rangeDef *KeyDef) bool {
	if rangeDef == nil {
		return true
	}
	av, ok := item[cond.attr]
	if !ok {
		return false
	}

	switch cond.op {
	case "=":
		return keyString(av) == keyString(cond.value)
	case "begins_with":
		if av.Type == dynago.TypeS && cond.value.Type == dynago.TypeS {
			return strings.HasPrefix(av.S, cond.value.S)
		}
		if av.Type == dynago.TypeB && cond.value.Type == dynago.TypeB {
			return len(av.B) >= len(cond.value.B) && string(av.B[:len(cond.value.B)]) == string(cond.value.B)
		}
		return false
	case "<":
		return compareKeyValues(av, cond.value) < 0
	case "<=":
		return compareKeyValues(av, cond.value) <= 0
	case ">":
		return compareKeyValues(av, cond.value) > 0
	case ">=":
		return compareKeyValues(av, cond.value) >= 0
	case "BETWEEN":
		return compareKeyValues(av, cond.value) >= 0 && compareKeyValues(av, cond.value2) <= 0
	default:
		return false
	}
}

// compareKeyValues compares two key AttributeValues, returning -1, 0, or 1.
func compareKeyValues(a, b dynago.AttributeValue) int {
	if a.Type != b.Type {
		return 0
	}
	switch a.Type {
	case dynago.TypeS:
		return strings.Compare(a.S, b.S)
	case dynago.TypeN:
		af, _ := parseFloat(a.N)
		bf, _ := parseFloat(b.N)
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	case dynago.TypeB:
		return strings.Compare(string(a.B), string(b.B))
	default:
		return 0
	}
}

func (m *MemoryBackend) Scan(_ context.Context, req *dynago.ScanRequest) (*dynago.ScanResponse, error) {
	td, err := m.table(req.TableName)
	if err != nil {
		return nil, err
	}
	td.mu.RLock()
	defer td.mu.RUnlock()

	var items map[string]map[string]map[string]dynago.AttributeValue
	var tableHashDef KeyDef
	var tableRangeDef *KeyDef

	if req.IndexName != "" {
		gsi, ok := td.gsis[req.IndexName]
		if !ok {
			return nil, &dynago.Error{
				Sentinel: dynago.ErrValidation,
				Message:  fmt.Sprintf("memdb: index %q does not exist on table %q", req.IndexName, req.TableName),
			}
		}
		items = gsi.items
		tableHashDef = gsi.schema.HashKey
		if gsi.schema.RangeKey != nil {
			tableRangeDef = gsi.schema.RangeKey
		}
	} else {
		items = td.items
		tableHashDef = td.schema.HashKey
		tableRangeDef = td.schema.RangeKey
	}

	// Collect all items in deterministic order for pagination.
	// Sort hash keys, then range keys within each hash.
	hashKeys := make([]string, 0, len(items))
	for hk := range items {
		hashKeys = append(hashKeys, hk)
	}
	sort.Strings(hashKeys)

	type itemRef struct {
		hashKey  string
		rangeKey string
	}
	var allRefs []itemRef
	for _, hk := range hashKeys {
		rangeMap := items[hk]
		rangeKeys := make([]string, 0, len(rangeMap))
		for rk := range rangeMap {
			rangeKeys = append(rangeKeys, rk)
		}
		sort.Strings(rangeKeys)
		for _, rk := range rangeKeys {
			allRefs = append(allRefs, itemRef{hashKey: hk, rangeKey: rk})
		}
	}

	// Find ExclusiveStartKey position.
	startIdx := 0
	if len(req.ExclusiveStartKey) > 0 {
		startHash := ""
		if hv, exists := req.ExclusiveStartKey[tableHashDef.Name]; exists {
			startHash = keyString(hv)
		}
		startRange := "_"
		if tableRangeDef != nil {
			if rv, exists := req.ExclusiveStartKey[tableRangeDef.Name]; exists {
				startRange = keyString(rv)
			}
		}
		for i, ref := range allRefs {
			if ref.hashKey == startHash && ref.rangeKey == startRange {
				startIdx = i + 1
				break
			}
		}
	}

	// Collect candidates from startIdx.
	type scanCandidate struct {
		item map[string]dynago.AttributeValue
	}
	var candidates []scanCandidate
	for i := startIdx; i < len(allRefs); i++ {
		ref := allRefs[i]
		candidates = append(candidates, scanCandidate{item: items[ref.hashKey][ref.rangeKey]})
	}

	// Apply Limit: DynamoDB Limit caps items evaluated (scanned), not results.
	limit := int(req.Limit)
	hasMore := false
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
		hasMore = true
	}

	scannedCount := int32(len(candidates))
	var resultItems []map[string]dynago.AttributeValue

	for _, c := range candidates {
		if req.FilterExpression != "" {
			matched, evalErr := evalCondition(req.FilterExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, c.item)
			if evalErr != nil {
				return nil, evalErr
			}
			if !matched {
				continue
			}
		}
		resultItem := deepCopyItem(c.item)
		resultItem = projectItem(resultItem, req.ProjectionExpression, req.ExpressionAttributeNames)
		resultItems = append(resultItems, resultItem)
	}

	resp := &dynago.ScanResponse{
		Items:        resultItems,
		Count:        int32(len(resultItems)),
		ScannedCount: scannedCount,
	}

	if hasMore && len(candidates) > 0 {
		lastItem := candidates[len(candidates)-1].item
		resp.LastEvaluatedKey = buildLastEvaluatedKey(lastItem, tableHashDef, tableRangeDef)
	}

	return resp, nil
}

func (m *MemoryBackend) BatchGetItem(_ context.Context, req *dynago.BatchGetItemRequest) (*dynago.BatchGetItemResponse, error) {
	// Count total keys across all tables.
	total := 0
	for _, kp := range req.RequestItems {
		total += len(kp.Keys)
	}
	if total > 100 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: BatchGetItem exceeds maximum of 100 keys"}
	}

	responses := make(map[string][]map[string]dynago.AttributeValue)

	for tableName, kp := range req.RequestItems {
		td, err := m.table(tableName)
		if err != nil {
			return nil, err
		}

		td.mu.RLock()
		var items []map[string]dynago.AttributeValue
		for _, key := range kp.Keys {
			hash, rng, err := td.extractKey(key)
			if err != nil {
				td.mu.RUnlock()
				return nil, err
			}
			item := td.getItem(hash, rng)
			if item != nil {
				result := deepCopyItem(item)
				result = projectItem(result, kp.ProjectionExpression, kp.ExpressionAttributeNames)
				items = append(items, result)
			}
			// Missing items are silently omitted.
		}
		td.mu.RUnlock()

		if len(items) > 0 {
			responses[tableName] = items
		}
	}

	return &dynago.BatchGetItemResponse{Responses: responses}, nil
}

func (m *MemoryBackend) BatchWriteItem(_ context.Context, req *dynago.BatchWriteItemRequest) (*dynago.BatchWriteItemResponse, error) {
	// Count total operations across all tables.
	total := 0
	for _, writes := range req.RequestItems {
		total += len(writes)
	}
	if total > 25 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: BatchWriteItem exceeds maximum of 25 operations"}
	}

	for tableName, writes := range req.RequestItems {
		td, err := m.table(tableName)
		if err != nil {
			return nil, err
		}

		td.mu.Lock()
		for _, wr := range writes {
			switch {
			case wr.PutItem != nil:
				hash, rng, err := td.extractKey(wr.PutItem.Item)
				if err != nil {
					td.mu.Unlock()
					return nil, err
				}
				oldItem := td.getItem(hash, rng)
				td.storeItem(hash, rng, wr.PutItem.Item)
				td.updateGSIs(oldItem, wr.PutItem.Item)

			case wr.DeleteItem != nil:
				hash, rng, err := td.extractKey(wr.DeleteItem.Key)
				if err != nil {
					td.mu.Unlock()
					return nil, err
				}
				oldItem := td.deleteItem(hash, rng)
				td.updateGSIs(oldItem, nil)
			}
		}
		td.mu.Unlock()
	}

	// No UnprocessedItems in memdb -- everything always succeeds.
	return &dynago.BatchWriteItemResponse{}, nil
}

func (m *MemoryBackend) TransactGetItems(_ context.Context, req *dynago.TransactGetItemsRequest) (*dynago.TransactGetItemsResponse, error) {
	if len(req.TransactItems) > 100 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: TransactGetItems exceeds maximum of 100 operations"}
	}

	// Resolve all tables up front.
	tables := make([]*tableData, len(req.TransactItems))
	for i, tgi := range req.TransactItems {
		td, err := m.table(tgi.TableName)
		if err != nil {
			return nil, err
		}
		tables[i] = td
	}

	// Lock all distinct tables for reading to get a consistent snapshot.
	locked := dedup(tables)
	for _, td := range locked {
		td.mu.RLock()
	}
	defer func() {
		for _, td := range locked {
			td.mu.RUnlock()
		}
	}()

	responses := make([]map[string]dynago.AttributeValue, len(req.TransactItems))
	for i, tgi := range req.TransactItems {
		td := tables[i]
		hash, rng, err := td.extractKey(tgi.Key)
		if err != nil {
			return nil, err
		}
		item := td.getItem(hash, rng)
		if item != nil {
			result := deepCopyItem(item)
			result = projectItem(result, tgi.ProjectionExpression, tgi.ExpressionAttributeNames)
			responses[i] = result
		}
	}

	return &dynago.TransactGetItemsResponse{Responses: responses}, nil
}

func (m *MemoryBackend) TransactWriteItems(_ context.Context, req *dynago.TransactWriteItemsRequest) (*dynago.TransactWriteItemsResponse, error) {
	if len(req.TransactItems) > 100 {
		return nil, &dynago.Error{Sentinel: dynago.ErrValidation, Message: "memdb: TransactWriteItems exceeds maximum of 100 operations"}
	}

	// Resolve all tables up front.
	tables := make([]*tableData, len(req.TransactItems))
	for i, twi := range req.TransactItems {
		tableName := txWriteTableName(twi)
		td, err := m.table(tableName)
		if err != nil {
			return nil, err
		}
		tables[i] = td
	}

	// Lock all distinct tables for writing (atomicity).
	locked := dedup(tables)
	for _, td := range locked {
		td.mu.Lock()
	}
	defer func() {
		for _, td := range locked {
			td.mu.Unlock()
		}
	}()

	// First pass: evaluate ALL conditions. Collect per-operation reasons.
	reasons := make([]dynago.TxCancelReason, len(req.TransactItems))
	anyFailed := false

	for i, twi := range req.TransactItems {
		td := tables[i]
		var condExpr string
		var names map[string]string
		var values map[string]dynago.AttributeValue
		var key map[string]dynago.AttributeValue

		switch {
		case twi.Put != nil:
			condExpr = twi.Put.ConditionExpression
			names = twi.Put.ExpressionAttributeNames
			values = twi.Put.ExpressionAttributeValues
			key = twi.Put.Item
		case twi.Delete != nil:
			condExpr = twi.Delete.ConditionExpression
			names = twi.Delete.ExpressionAttributeNames
			values = twi.Delete.ExpressionAttributeValues
			key = twi.Delete.Key
		case twi.Update != nil:
			condExpr = twi.Update.ConditionExpression
			names = twi.Update.ExpressionAttributeNames
			values = twi.Update.ExpressionAttributeValues
			key = twi.Update.Key
		case twi.ConditionCheck != nil:
			condExpr = twi.ConditionCheck.ConditionExpression
			names = twi.ConditionCheck.ExpressionAttributeNames
			values = twi.ConditionCheck.ExpressionAttributeValues
			key = twi.ConditionCheck.Key
		}

		if condExpr == "" {
			continue
		}

		hash, rng, err := td.extractKey(key)
		if err != nil {
			return nil, err
		}

		existing := td.getItem(hash, rng)
		if existing == nil {
			existing = map[string]dynago.AttributeValue{}
		}

		ok, evalErr := evalCondition(condExpr, names, values, existing)
		if evalErr != nil {
			return nil, evalErr
		}
		if !ok {
			reasons[i] = dynago.TxCancelReason{Code: "ConditionalCheckFailed", Message: "condition check failed"}
			anyFailed = true
		}
	}

	if anyFailed {
		return nil, &dynago.TxCancelledError{Reasons: reasons}
	}

	// Second pass: apply all writes atomically.
	for i, twi := range req.TransactItems {
		td := tables[i]
		switch {
		case twi.Put != nil:
			p := twi.Put
			hash, rng, err := td.extractKey(p.Item)
			if err != nil {
				return nil, err
			}
			oldItem := td.getItem(hash, rng)
			td.storeItem(hash, rng, p.Item)
			td.updateGSIs(oldItem, p.Item)

		case twi.Delete != nil:
			d := twi.Delete
			hash, rng, err := td.extractKey(d.Key)
			if err != nil {
				return nil, err
			}
			oldItem := td.deleteItem(hash, rng)
			td.updateGSIs(oldItem, nil)

		case twi.Update != nil:
			u := twi.Update
			hash, rng, err := td.extractKey(u.Key)
			if err != nil {
				return nil, err
			}

			existing := td.getItem(hash, rng)
			var item map[string]dynago.AttributeValue
			if existing != nil {
				item = deepCopyItem(existing)
			} else {
				item = deepCopyItem(u.Key)
			}

			if u.UpdateExpression != "" {
				nodes, parseErr := parseUpdateExpression(u.UpdateExpression, u.ExpressionAttributeNames, u.ExpressionAttributeValues)
				if parseErr != nil {
					return nil, parseErr
				}
				updated, evalErr := evalUpdateNodes(nodes, item)
				if evalErr != nil {
					return nil, evalErr
				}
				item = updated
			}

			td.storeItem(hash, rng, item)
			td.updateGSIs(existing, item)

		case twi.ConditionCheck != nil:
			// No data modification.
		}
	}

	return &dynago.TransactWriteItemsResponse{}, nil
}

// txWriteTableName extracts the table name from a TransactWriteItem.
func txWriteTableName(twi dynago.TransactWriteItem) string {
	switch {
	case twi.Put != nil:
		return twi.Put.TableName
	case twi.Delete != nil:
		return twi.Delete.TableName
	case twi.Update != nil:
		return twi.Update.TableName
	case twi.ConditionCheck != nil:
		return twi.ConditionCheck.TableName
	default:
		return ""
	}
}

// dedup returns a deduplicated slice of tableData pointers preserving order.
func dedup(tables []*tableData) []*tableData {
	seen := make(map[*tableData]bool, len(tables))
	var out []*tableData
	for _, td := range tables {
		if !seen[td] {
			seen[td] = true
			out = append(out, td)
		}
	}
	return out
}

// compile-time check
var _ dynago.Backend = (*MemoryBackend)(nil)
