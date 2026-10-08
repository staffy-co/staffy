package domain

import (
        "os"
        "path/filepath"
        "regexp"
        "sort"
        "testing"
        "time"
)

// createTablePattern extracts "schema.table" from every CREATE TABLE in the
// migration tree. Newlines inside the parenthesis list are tolerated.
var createTablePattern = regexp.MustCompile(`(?im)^CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_]+\.[a-z_]+)\s*\(`)

// migrationDirs is the exhaustive, ordered list of migration directories. A
// new directory must be added here AND claimed by the class dictionary.
var migrationDirs = []string{
        "../../../migrations/identity",
        "../../../migrations/employee",
        "../../../migrations/connector",
        "../../../migrations/privacy",
        "../../../migrations/cases",
}

func migrationTables(t *testing.T) []string {
        t.Helper()
        seen := map[string]bool{}
        for _, dir := range migrationDirs {
                files, err := os.ReadDir(dir)
                if err != nil {
                        t.Fatalf("read migration dir %s: %v", dir, err)
                }
                for _, file := range files {
                        if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
                                continue
                        }
                        raw, err := os.ReadFile(filepath.Join(dir, file.Name()))
                        if err != nil {
                                t.Fatalf("read migration %s: %v", file.Name(), err)
                        }
                        // Only the goose Up part counts: the Down section drops tables.
                        up := raw
                        if marker := regexp.MustCompile(`(?m)^--\s*\+goose\s+Down`).FindIndex(raw); marker != nil {
                                up = raw[:marker[0]]
                        }
                        for _, match := range createTablePattern.FindAllStringSubmatch(string(up), -1) {
                                table := match[1]
                                if table == "privacy.data_map_entries" || table == "privacy.retention_policies" ||
                                        table == "privacy.legal_hold_events" || table == "privacy.deletion_requests" ||
                                        table == "privacy.deletion_outcomes" || table == "privacy.lifecycle_receipts" {
                                        continue // governance tables belong to the map itself, not to mapped data classes
                                }
                                if seen[table] {
                                        t.Fatalf("table %s is created by more than one migration", table)
                                }
                                seen[table] = true
                        }
                }
        }
        tables := make([]string, 0, len(seen))
        for table := range seen {
                tables = append(tables, table)
        }
        sort.Strings(tables)
        return tables
}

// TestPerimeterIntegrity keeps the class dictionary synchronized with the
// physical database: every migrated table must be claimed by at least one
// class (unregistered data is impossible), and every class must reference at
// least one table that a migration actually creates (no phantom classes).
func TestPerimeterIntegrity(t *testing.T) {
        tables := migrationTables(t)
        owners := TableOwners()

        unclaimed := []string{}
        for _, table := range tables {
                if len(owners[table]) == 0 {
                        unclaimed = append(unclaimed, table)
                }
        }
        if len(unclaimed) > 0 {
                t.Fatalf("migrations created tables that no artifact class claims (unregistered data): %v", unclaimed)
        }

        phantom := []string{}
        for _, spec := range ClassSpecs() {
                exists := false
                for _, table := range spec.Tables {
                        claimed := false
                        for _, candidate := range tables {
                                if candidate == table {
                                        claimed = true
                                        break
                                }
                        }
                        if !claimed {
                                t.Fatalf("class %s references table %s that no migration creates", spec.ID, table)
                        }
                        exists = true
                }
                if !exists {
                        phantom = append(phantom, string(spec.ID))
                }
        }
        if len(phantom) > 0 {
                t.Fatalf("classes without any physical table: %v", phantom)
        }
}

func TestClassDictionaryShape(t *testing.T) {
        if got := len(AllClasses()); got != 20 {
                t.Fatalf("dictionary size = %d, want 20", got)
        }
        // Every class is resolvable and has a title and at least one table.
        for _, class := range AllClasses() {
                spec, err := LookupClass(class)
                if err != nil {
                        t.Fatalf("lookup %s: %v", class, err)
                }
                if spec.Title == "" || len(spec.Tables) == 0 {
                        t.Fatalf("class %s has no title or tables", class)
                }
        }
        if _, err := LookupClass("not-a-class"); err == nil {
                t.Fatal("unknown class must be rejected")
        }
}

func TestMapEntryCompleteness(t *testing.T) {
        complete := DataMapEntry{
                Purpose: "Обработка обращений", LegalBasis: "Договор", Controller: "ООО Ромашка",
                Processor: "Staffy", Recipients: []string{"support"}, SubProcessors: []string{"hosting"},
                Locations: []string{"EU"}, AccessibleRoles: []string{"owner"},
                KnownCopies: []string{"postgres"}, RetentionPolicy: "ret-001",
                DeletionMethod: "purge", ExportMethod: "export",
        }
        if complete.Status() != MapStatusComplete {
                t.Fatalf("complete entry reported %s", complete.Status())
        }
        incomplete := complete
        incomplete.LegalBasis = ""
        incomplete.KnownCopies = nil
        if incomplete.Status() != MapStatusIncomplete {
                t.Fatalf("incomplete entry reported %s", incomplete.Status())
        }
        missing := incomplete.MissingAttributes()
        if len(missing) != 2 || missing[0] != "legal_basis" || missing[1] != "known_copies" {
                t.Fatalf("missing attributes = %v, want [legal_basis known_copies]", missing)
        }
}

func TestPolicyTerms(t *testing.T) {
        policy := RetentionPolicy{ActiveUseDays: 30, ArchiveDays: 7}
        anchor := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
        due := policy.DeletionDueAt(anchor)
        if want := anchor.Add(37 * 24 * time.Hour); !due.Equal(want) {
                t.Fatalf("due = %v, want %v", due, want)
        }
        mandatory := policy.MandatoryDueAt(anchor)
        if want := anchor.Add(0 * 24 * time.Hour); !mandatory.Equal(want) {
                t.Fatalf("mandatory due = %v, want %v", mandatory, want)
        }
}

func TestVoicePreviewInvariant(t *testing.T) {
        ok := RetentionPolicy{Class: ClassVoicePreviewMedia, ActiveUseDays: 1, ArchiveDays: 0}
        if !ok.HonorsVoicePreviewInvariant() {
                t.Fatal("24h-conforming preview policy must pass")
        }
        tooLong := RetentionPolicy{Class: ClassVoicePreviewMedia, ActiveUseDays: 5, ArchiveDays: 0}
        if tooLong.HonorsVoicePreviewInvariant() {
                t.Fatal("policy promising more than the 24h purge invariant must be rejected")
        }
        other := RetentionPolicy{Class: ClassKnowledgeSourcePayload, ActiveUseDays: 90, ArchiveDays: 30}
        if !other.HonorsVoicePreviewInvariant() {
                t.Fatal("invariant only applies to the preview media class")
        }
}

func TestDeriveHolds(t *testing.T) {
        now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
        events := []LegalHoldEvent{
                {Event: "activated", Classes: []ArtifactClass{ClassKnowledgeSourcePayload}, Basis: "Судебный запрос", Approver: "Юрист", ReviewDeadline: now.Add(24 * time.Hour), CreatedAt: now.Add(-48 * time.Hour)},
                {Event: "released", Classes: []ArtifactClass{ClassEmployeeDraft}, Basis: "Снято", Approver: "Юрист", ReviewDeadline: now.Add(-1 * time.Hour), CreatedAt: now.Add(-24 * time.Hour)},
        }
        states := DeriveHolds(events, now)
        knowledge := states[ClassKnowledgeSourcePayload]
        if knowledge.Status != HoldActive {
                t.Fatalf("knowledge hold = %s, want active", knowledge.Status)
        }
        if knowledge.Approver != "Юрист" {
                t.Fatalf("approver = %s", knowledge.Approver)
        }
        drafts := states[ClassEmployeeDraft]
        if drafts.Status != HoldNone {
                t.Fatalf("released hold = %s, want none", drafts.Status)
        }
        // Expired review deadline requires a decision, it never stays "active".
        expired := []LegalHoldEvent{
                {Event: "activated", Classes: []ArtifactClass{ClassAuditTrail}, Basis: "Спор", Approver: "Юрист", ReviewDeadline: now.Add(-2 * time.Hour), CreatedAt: now.Add(-72 * time.Hour)},
        }
        if state := DeriveHolds(expired, now)[ClassAuditTrail]; state.Status != HoldRequiresDecision {
                t.Fatalf("expired hold = %s, want requires_decision", state.Status)
        }
}
