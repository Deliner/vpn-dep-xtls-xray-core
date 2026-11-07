package documents

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/repository"
)

const Research = "docs/research/2026-11-russia-vpn"
const Inventory = Research + "/00-control/inventory.md"

func InventoryContent(root string) ([]byte, error) {
	base := filepath.Join(root, Research)
	paths, err := repository.Files(base, ".md")
	if err != nil {
		return nil, err
	}
	var content bytes.Buffer
	fmt.Fprint(&content, "# Инвентаризация исследовательского дерева\n\n")
	fmt.Fprintln(&content, "Создаётся командой `just research-inventory`; проверяется `just docs-check`.")
	fmt.Fprintln(&content, "Реорганизация путей не обновляет даты и выводы исследования. Архивные снимки сохраняются побайтно.")
	fmt.Fprintf(&content, "Markdown-файлов: **%d**, записей hashes: **%d**. Inventory исключён из собственного перечня.\n\n", len(paths), len(paths)-1)
	fmt.Fprintln(&content, "| Файл | Bytes | SHA-256 |\n|---|---:|---|")
	for _, path := range paths {
		if path == filepath.Join(root, Inventory) {
			continue
		}
		row, err := inventoryRow(base, path)
		if err != nil {
			return nil, err
		}
		content.WriteString(row)
	}
	return content.Bytes(), nil
}

func inventoryRow(base, path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return "", err
	}
	link, err := filepath.Rel(filepath.Join(base, "00-control"), path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("| [%s](%s) | %d | `%x` |\n", filepath.ToSlash(relative), filepath.ToSlash(link), len(raw), sha256.Sum256(raw)), nil
}

func WriteInventory(root string) error {
	content, err := InventoryContent(root)
	if err != nil {
		return err
	}
	return safeio.Atomic(filepath.Join(root, Inventory), content, 0644)
}

func CheckInventory(root string) error {
	expected, err := InventoryContent(root)
	if err != nil {
		return err
	}
	actual, err := os.ReadFile(filepath.Join(root, Inventory))
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("research inventory is stale; run just research-inventory")
	}
	return nil
}
