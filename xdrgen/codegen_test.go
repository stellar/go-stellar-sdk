package main

import "testing"

func TestSnakeToCamel(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"account_entry", "AccountEntry"},
		{"LIQUIDITY_POOL_FEE_V18", "LiquidityPoolFeeV18"},
		{"SC_VAL", "ScVal"},
		{"int", "Int32"},
		{"unsigned int", "Uint32"},
		{"hyper", "Int64"},
		{"unsigned hyper", "Uint64"},
		{"float", "Float32"},
		{"double", "Float64"},
		{"bool", "Bool"},
		{"", ""},
		{"a", "A"},
		{"SCPStatement", "ScpStatement"},
		{"AccountID", "AccountId"},
		{"SHA256Hash", "Sha256Hash"},
		{"uint256", "Uint256"},
		{"HmacSha256Key", "HmacSha256Key"},
	}
	for _, tt := range tests {
		if got := snakeToCamel(tt.in); got != tt.want {
			t.Errorf("snakeToCamel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAcronymSplit(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"SCPStatement", "SCP_Statement"},
		{"AccountEntry", "Account_Entry"},
		{"SHA256Hash", "SHA256_Hash"},
		{"AccountID", "Account_ID"},
		{"LedgerCloseMetaV0", "Ledger_Close_Meta_V0"},
		{"simple", "simple"},
		{"ABC", "ABC"},
		{"ABCDef", "ABC_Def"},
	}
	for _, tt := range tests {
		if got := acronymSplit(tt.in); got != tt.want {
			t.Errorf("acronymSplit(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGoTypeName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"AccountEntry", "AccountEntry"},
		{"ACCOUNT_ENTRY", "AccountEntry"},
		{"account_entry", "AccountEntry"},
		{"SCPStatementType", "ScpStatementType"},
		{"SC_VAL", "ScVal"},
	}
	for _, tt := range tests {
		if got := GoTypeName(tt.in); got != tt.want {
			t.Errorf("GoTypeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
