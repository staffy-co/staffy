package tools

import (
        "crypto/sha256"
        "embed"
        "encoding/hex"
        "encoding/json"
        "fmt"
        "io/fs"
        "strconv"
        "strings"
)

// The tool registry is the versioned artifact store of the published typed
// tool contracts (Story 7-2, the registry discipline of Story 7-1): every
// published contract is a JSON artifact pinned by its sha256 in the
// append-only index, embedded into the binary and validated BEFORE any
// use. The loader is fail-closed: one invalid artifact fails the WHOLE
// registry — a partial tool dictionary never exists, the CI catches the
// break before deploy, and the runtime panics at load instead of serving
// half a dictionary. Published versions are immutable: touching an
// artifact's bytes changes its sha256 and fails the pin; a new version is
// a new file plus a new index entry.

//go:embed registry/*.json
var registryFiles embed.FS

const registryDir = "registry"
const registryIndexFile = registryDir + "/index.json"

// indexEntry pins one published tool artifact: the contract id, the
// version, the artifact file name inside registry/ and the sha256 of its
// exact bytes.
type indexEntry struct {
        ID      string `json:"id"`
        Version string `json:"version"`
        File    string `json:"file"`
        SHA256  string `json:"sha256"`
}

type registryIndex struct {
        Entries []indexEntry `json:"entries"`
}

// The platform dictionaries: a tool artifact USES these words, it never
// extends them — a value outside a dictionary is a load-time rejection
// (extending a dictionary is a core change, never an artifact change).
//
// determinism_class carries the SINGLE value deterministic_read: a
// write-class tool is structurally impossible — the machine-backed
// guarantee of AD-6/AD-7 («executable writes never enter the model's tool
// set»). Adding a second class is a core change with an explicit
// architecture decision, never a registry edit.
var (
        determinismClassDictionary = map[string]bool{"deterministic_read": true}
        adapterStatusDictionary    = map[string]bool{"implemented": true, "not_implemented": true}
        toolFieldTypeDictionary    = map[string]bool{"string": true, "integer": true, "boolean": true}
)

// loadRegistry reads the index and every tool artifact from fsys, verifies
// the sha256 pins, validates every contract and the cross-artifact rules,
// and returns the contracts in the canonical index order — or the first
// named reason the registry is not loadable.
func loadRegistry(fsys fs.FS) ([]ToolContract, error) {
        indexRaw, err := fs.ReadFile(fsys, registryIndexFile)
        if err != nil {
                return nil, fmt.Errorf("индекс реестра tools: %w", err)
        }
        var index registryIndex
        if err := json.Unmarshal(indexRaw, &index); err != nil {
                return nil, fmt.Errorf("индекс реестра tools: JSON: %v", err)
        }
        if len(index.Entries) == 0 {
                return nil, fmt.Errorf("индекс реестра tools: записи отсутствуют")
        }

        contracts := make([]ToolContract, 0, len(index.Entries))
        referenced := map[string]bool{}
        seen := map[string]bool{}
        for _, entry := range index.Entries {
                key := entry.ID + "@" + entry.Version
                if seen[key] || referenced[entry.File] {
                        return nil, fmt.Errorf("индексная запись %s: дубликат", key)
                }
                seen[key] = true
                if entry.File != key+".json" {
                        return nil, fmt.Errorf("индексная запись %s: имя файла %s нарушает канон <id>@<version>.json", key, entry.File)
                }
                path := registryDir + "/" + entry.File
                raw, err := fs.ReadFile(fsys, path)
                if err != nil {
                        return nil, fmt.Errorf("%s: %v", key, err)
                }
                sum := sha256.Sum256(raw)
                if hex.EncodeToString(sum[:]) != entry.SHA256 {
                        return nil, fmt.Errorf("%s: артефакт изменён после индексации (sha256 не совпадает)", key)
                }
                var contract ToolContract
                if err := json.Unmarshal(raw, &contract); err != nil {
                        return nil, fmt.Errorf("%s: JSON: %v", key, err)
                }
                if err := validateContract(contract); err != nil {
                        return nil, err
                }
                contracts = append(contracts, contract)
                referenced[entry.File] = true
        }

        // Every artifact file must be indexed: an unindexed file means the
        // registry and the directory disagree — an author forgot the index
        // entry, and fail-closed refuses the ambiguity.
        entries, err := fs.ReadDir(fsys, registryDir)
        if err != nil {
                return nil, fmt.Errorf("реестр tools: %v", err)
        }
        for _, entry := range entries {
                if entry.IsDir() || entry.Name() == "index.json" || !strings.HasSuffix(entry.Name(), ".json") {
                        continue
                }
                if !referenced[entry.Name()] {
                        return nil, fmt.Errorf("файл tools/%s не индексирован", entry.Name())
                }
        }
        return contracts, nil
}

// validateContract checks the single-artifact rules: the mandatory fields
// (the ValidateVariants lesson of 5-1 — EVERY unprotected field is
// mandatory), the platform dictionaries and the schema shape.
func validateContract(c ToolContract) error {
        key := c.ID + "@" + c.Version
        if c.ID == "" || c.Version == "" {
                return fmt.Errorf("артефакт tools %s: обязательные поля id/version отсутствуют", key)
        }
        for _, field := range []struct {
                name  string
                empty bool
        }{
                {"name", c.Name == ""},
                {"purpose", c.Purpose == ""},
                {"determinism_class", c.DeterminismClass == ""},
                {"input_schema", len(c.InputSchema) == 0},
                {"output_schema", len(c.OutputSchema) == 0},
                {"owner", c.Owner == ""},
                {"evidence_ref", c.EvidenceRef == ""},
                {"adapter_status", c.AdapterStatus == ""},
        } {
                if field.empty {
                        return fmt.Errorf("%s: обязательное поле %s отсутствует", key, field.name)
                }
        }
        if _, _, _, err := parseSemver(c.Version); err != nil {
                return fmt.Errorf("%s: %v", key, err)
        }
        if !determinismClassDictionary[c.DeterminismClass] {
                return fmt.Errorf("%s: determinism_class: значение %s вне платформенного словаря", key, c.DeterminismClass)
        }
        if !adapterStatusDictionary[c.AdapterStatus] {
                return fmt.Errorf("%s: adapter_status: значение %s вне платформенного словаря", key, c.AdapterStatus)
        }
        for _, schema := range []struct {
                name   string
                fields map[string]ToolSchemaField
        }{
                {"input_schema", c.InputSchema},
                {"output_schema", c.OutputSchema},
        } {
                for name, field := range schema.fields {
                        if field.Label == "" {
                                return fmt.Errorf("%s: %s.%s: label отсутствует", key, schema.name, name)
                        }
                        if !toolFieldTypeDictionary[field.Type] {
                                return fmt.Errorf("%s: %s.%s: тип %s вне платформенного словаря", key, schema.name, name, field.Type)
                        }
                        if field.Type == "integer" && len(field.Values) > 0 {
                                return fmt.Errorf("%s: %s.%s: integer-ключ не может объявлять values", key, schema.name, name)
                        }
                        if field.Type != "integer" && (field.Min != 0 || field.Max != 0) {
                                return fmt.Errorf("%s: %s.%s: границы min/max допустимы только у integer-ключа", key, schema.name, name)
                        }
                        if field.Type != "string" && len(field.Values) > 0 {
                                return fmt.Errorf("%s: %s.%s: values допустимы только у string-ключа", key, schema.name, name)
                        }
                }
        }
        if c.Dependency != nil && c.Dependency.ConnectorType == "" {
                return fmt.Errorf("%s: dependency: обязательное поле connector_type отсутствует", key)
        }
        return nil
}

// parseSemver accepts the strict semantic core major.minor.patch — the
// registry publishes released versions only, without prerelease or build
// tags (an honest boundary of the slice, extendable by a core change).
func parseSemver(version string) (int, int, int, error) {
        parts := strings.Split(version, ".")
        if len(parts) != 3 {
                return 0, 0, 0, fmt.Errorf("неверный формат версии %s (ожидается major.minor.patch)", version)
        }
        numbers := make([]int, 3)
        for i, part := range parts {
                number, err := strconv.Atoi(part)
                if err != nil || number < 0 || (len(part) > 1 && part[0] == '0') {
                        return 0, 0, 0, fmt.Errorf("неверный формат версии %s (ожидается major.minor.patch)", version)
                }
                numbers[i] = number
        }
        return numbers[0], numbers[1], numbers[2], nil
}
