import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

// Some npm packages ship Go examples without a nested go.mod. Without this
// boundary, `go test ./...` treats those examples as SheetProof source code.
const boundary = join("node_modules", "flatted", "golang", "go.mod");
if (existsSync(dirname(boundary))) {
  mkdirSync(dirname(boundary), { recursive: true });
  writeFileSync(boundary, "module example.com/flatted-go-example\n\ngo 1.24.0\n");
}
