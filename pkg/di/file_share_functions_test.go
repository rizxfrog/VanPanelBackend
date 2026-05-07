package di

import (
	"os"
	"strings"
	"testing"
)

func TestVerifyShareAccessReturnsBigintShareID(t *testing.T) {
	content, err := os.ReadFile("../../scripts/sql/file_share_functions.sql")
	if err != nil {
		t.Fatalf("read file_share_functions.sql failed: %v", err)
	}

	sql := string(content)
	start := strings.Index(sql, "CREATE OR REPLACE FUNCTION verify_share_access")
	if start == -1 {
		t.Fatal("verify_share_access function not found")
	}

	fragment := sql[start:]
	end := strings.Index(fragment, "$$ LANGUAGE plpgsql;")
	if end == -1 {
		t.Fatal("verify_share_access function end not found")
	}
	fragment = fragment[:end]

	if !strings.Contains(fragment, "share_id BIGINT") {
		t.Fatal("verify_share_access share_id must be BIGINT to match cl_file_shares.id")
	}
}

func TestVerifyShareAccessDoesNotReturnNullStrings(t *testing.T) {
	content, err := os.ReadFile("../../scripts/sql/file_share_functions.sql")
	if err != nil {
		t.Fatalf("read file_share_functions.sql failed: %v", err)
	}

	sql := string(content)
	start := strings.Index(sql, "CREATE OR REPLACE FUNCTION verify_share_access")
	if start == -1 {
		t.Fatal("verify_share_access function not found")
	}

	fragment := sql[start:]
	end := strings.Index(fragment, "$$ LANGUAGE plpgsql;")
	if end == -1 {
		t.Fatal("verify_share_access function end not found")
	}
	fragment = fragment[:end]

	if strings.Contains(fragment, "NULL::VARCHAR") {
		t.Fatal("verify_share_access must not return NULL string columns scanned into Go strings")
	}
	if !strings.Contains(fragment, "''::VARCHAR(20)") {
		t.Fatal("verify_share_access should return empty access_level when absent")
	}
	if !strings.Contains(fragment, "''::VARCHAR(100)") {
		t.Fatal("verify_share_access should return empty error_message when access is valid")
	}
}
