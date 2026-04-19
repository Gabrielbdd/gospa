// Re-exports the generated public runtime config so application code imports
// from a stable path. The file behind ./gen/runtime-config is produced by
// `gofra generate config -ts-out web/src/gen` (wired into `mise run generate`).
export { runtimeConfig, loadRuntimeConfig } from "@/gen/runtime-config";
export type { RuntimeConfig } from "@/gen/runtime-config";
