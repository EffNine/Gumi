# Novexa Plugin System Specification

Version: 1.0  
Status: Draft  
Scope: Plugin architecture, hooks, permissions, lifecycle, and extensibility model for Novexa Runtime

---

# 1. Purpose

This document defines the Plugin System for Novexa Runtime.

The Plugin System allows Novexa to be extended without bloating the core runtime.

Plugins can add or modify behaviour in areas such as:

- context processing
- prompt optimization
- validation
- repair
- telemetry
- provider support
- dashboard panels
- model profiles
- workflow hooks
- future agent behaviour

Plugins make Novexa extensible, community-driven, and marketplace-ready.

---

# 2. Core Philosophy

Novexa core should remain small, stable, and predictable.

Plugins should provide optional capability.

```text
Core Runtime = stable foundation
Plugins = optional extensions
```

If a feature is useful but not essential for the core runtime, it should probably be a plugin.

---

# 3. Plugin System Goals

The Plugin System must be:

- safe
- modular
- inspectable
- versioned
- permission-based
- local-first
- easy to develop
- easy to disable
- easy to debug
- marketplace-ready later

---

# 4. Plugin System Non-Goals for V1

V1 does not need:

- public marketplace
- paid plugins
- remote plugin execution
- cloud plugin registry
- complex dependency solver
- untrusted network plugin execution
- enterprise plugin policy management

V1 should design for these later, but implementation can stay simple.

---

# 5. Plugin Position in Runtime

Plugins attach to the Pipeline through hooks.

```text
Pipeline Engine
    ↓
Hook
    ↓
Plugin Engine
    ↓
Plugin
    ↓
Hook Result
    ↓
Pipeline Engine
```

Plugins must not bypass the Pipeline Engine.

---

# 6. Plugin Architecture Overview

```text
Novexa Runtime
├── Plugin Engine
│   ├── Plugin Discovery
│   ├── Manifest Validator
│   ├── Permission Manager
│   ├── Hook Registry
│   ├── Plugin Loader
│   ├── Plugin Runtime
│   └── Plugin Telemetry
│
└── Plugins
    ├── Context Plugins
    ├── Prompt Plugins
    ├── Validation Plugins
    ├── Repair Plugins
    ├── Provider Plugins
    ├── Telemetry Plugins
    └── Dashboard Plugins
```

---

# 7. Plugin Types

Supported conceptual plugin types:

```text
context
prompt
guard
validation
repair
provider
model_profile
telemetry
dashboard
cli
agent
```

V1 implementation priority:

```text
1. prompt
2. validation
3. repair
4. model_profile
5. provider
6. telemetry
```

Context and dashboard plugins can be implemented later if needed.

---

# 8. Plugin Hooks

Plugins attach to defined hooks.

Hooks are stable extension points in the Pipeline.

---

## 8.1 Core Hooks

```text
before_request
after_request_normalized

before_context
after_context

before_memory
after_memory

before_prompt
after_prompt

before_guard
after_guard

before_provider
after_provider

before_validation
after_validation

before_repair
after_repair

before_response
after_response

on_error
```

---

## 8.2 Hook Order

Default pipeline hook order:

```text
before_request
after_request_normalized
before_context
after_context
before_memory
after_memory
before_prompt
after_prompt
before_guard
after_guard
before_provider
after_provider
before_validation
after_validation
before_repair
after_repair
before_response
after_response
on_error
```

---

## 8.3 Hook Execution Rule

Hooks should execute in deterministic order.

Order priority:

```text
1. Required core plugins
2. User-installed plugins by priority
3. Plugins with same priority sorted by name
```

---

# 9. Plugin Manifest

Every plugin must include a manifest file.

Default manifest name:

```text
novexa.plugin.yaml
```

---

## 9.1 Manifest Example

```yaml
id: better-json
name: Better JSON Repair
version: 0.1.0
description: Improves JSON extraction and repair for local models.

type: repair

entry:
  runtime: wasm
  path: plugin.wasm

author:
  name: Novexa Community
  url: https://example.com

compatibility:
  novexa_min_version: 0.1.0
  novexa_max_version: 0.x
  schema_version: 1

hooks:
  - before_repair
  - after_validation

permissions:
  - read_response
  - modify_response
  - read_validation_report
  - emit_telemetry

config_schema:
  max_repair_size:
    type: number
    default: 100000
  strict_mode:
    type: boolean
    default: true

defaults:
  enabled: true
  priority: 50
  required: false
```

---

# 10. Required Manifest Fields

```text
id
name
version
type
entry
compatibility
hooks
permissions
defaults
```

Optional fields:

```text
description
author
homepage
repository
license
config_schema
keywords
tags
```

---

# 11. Plugin ID Rules

Plugin IDs must use lowercase kebab-case.

Examples:

```text
better-json
qwen-prompt-style
context-deduper
ollama-plus
markdown-validator
```

Invalid:

```text
Better JSON
better_json
better.json
```

---

# 12. Plugin Versioning

Plugins must use semantic versioning.

Example:

```text
0.1.0
1.0.0
1.2.3
```

Breaking changes require major version increment.

---

# 13. Plugin Compatibility

Manifest compatibility:

```yaml
compatibility:
  novexa_min_version: 0.1.0
  novexa_max_version: 0.x
  schema_version: 1
```

Novexa must reject incompatible plugins with clear error.

---

# 14. Plugin Permissions

Plugins must declare permissions.

Default policy:

```text
deny by default
```

A plugin can only access what its permissions allow.

---

## 14.1 Permission Categories

```text
read_request
modify_request

read_context
modify_context

read_prompt
modify_prompt

read_guard_report
modify_guard_constraints

read_provider_request
modify_provider_request

read_provider_response
modify_provider_response

read_response
modify_response

read_validation_report
modify_validation_report

read_config
modify_config

read_memory
write_memory

emit_telemetry

network_access
filesystem_read
filesystem_write

register_provider
register_dashboard_panel
register_cli_command
```

---

## 14.2 Dangerous Permissions

The following are dangerous and should require explicit user approval:

```text
network_access
filesystem_write
modify_config
write_memory
register_provider
modify_provider_request
modify_provider_response
```

---

## 14.3 V1 Permission Rule

For V1, plugin permissions can be enforced at a simple runtime level.

Future versions should support stronger sandboxing.

---

# 15. Plugin Runtime Options

Possible plugin runtimes:

```text
native
wasm
process
script
```

---

## 15.1 Native Plugin

Compiled into Novexa or loaded as internal module.

Pros:

- fastest
- simplest for core plugins

Cons:

- less isolated
- harder for third-party plugins

---

## 15.2 WASM Plugin

Recommended long-term plugin runtime.

Pros:

- portable
- sandboxable
- safer
- language-agnostic

Cons:

- more complex
- needs host API design

---

## 15.3 Process Plugin

Plugin runs as local subprocess.

Pros:

- easier isolation
- can be written in any language

Cons:

- slower
- harder lifecycle management

---

## 15.4 Script Plugin

Plugin runs as interpreted script.

Pros:

- easy for users

Cons:

- security risk
- harder sandboxing

---

# 16. V1 Runtime Recommendation

For V1:

```text
Core plugins = native/internal
Third-party plugin design = manifest + hook interface
WASM support = planned
```

This allows Novexa to design the plugin system without overbuilding execution sandboxing too early.

---

# 17. Plugin Lifecycle

Plugin lifecycle:

```text
discover
validate_manifest
check_compatibility
check_permissions
load_config
initialize
register_hooks
ready
execute_hooks
shutdown
```

---

## 17.1 Discover

Novexa scans plugin directories.

Default:

```text
./plugins
~/.novexa/plugins
```

---

## 17.2 Validate Manifest

Novexa validates:

- required fields
- valid ID
- valid version
- valid type
- valid hooks
- valid permissions
- valid entry path
- compatible Novexa version

---

## 17.3 Check Permissions

Novexa compares requested permissions against policy.

If dangerous permissions are requested:

```text
require explicit approval
```

---

## 17.4 Initialize

Plugin receives initialization context.

```text
PluginInitContext
├── plugin_id
├── plugin_config
├── novexa_version
├── workspace_id
├── permissions
└── runtime_info
```

---

## 17.5 Register Hooks

Plugin registers one or more hook handlers.

---

## 17.6 Execute Hooks

During pipeline execution, Plugin Engine calls matching plugin hooks.

---

## 17.7 Shutdown

Plugin cleans up resources.

---

# 18. Plugin Hook Input

Plugins receive a restricted hook input.

```text
PluginHookInput
├── hook_name
├── request_id
├── workspace_id
├── session_id
├── runtime_mode
├── allowed_context
├── plugin_config
└── metadata
```

`allowed_context` depends on plugin permissions.

---

# 19. Plugin Hook Output

```text
PluginHookResult
├── status
├── changes
├── warnings
├── errors
├── telemetry_events
└── metadata
```

Allowed status:

```text
success
skipped
warning
failed_recoverable
failed_fatal
```

---

# 20. Plugin Change Format

Plugins should return patch-like changes.

Example:

```yaml
changes:
  - op: replace
    path: prompt.system_prompt
    value: "Updated system prompt..."
```

Supported operations:

```text
add
replace
remove
append
merge
```

Novexa applies changes only if permission allows.

---

# 21. Plugin Failure Behaviour

Plugin failure must not crash the runtime by default.

Default:

```text
plugin error → telemetry warning → continue pipeline
```

If plugin is marked as required:

```yaml
defaults:
  required: true
```

Then plugin failure may stop the pipeline.

---

# 22. Required Plugin Rule

Required plugins should be used sparingly.

Examples:

- enterprise compliance plugin
- strict validation plugin
- workspace policy plugin

For local V1, most plugins should not be required.

---

# 23. Plugin Priority

Plugins may define priority.

```yaml
defaults:
  priority: 50
```

Priority range:

```text
0 - 100
```

Higher priority runs earlier.

If same priority, sort by plugin ID.

---

# 24. Plugin Configuration

Plugins may define a config schema.

Example:

```yaml
config_schema:
  strict_mode:
    type: boolean
    default: true

  max_size:
    type: number
    default: 100000
```

User config:

```yaml
plugins:
  better-json:
    enabled: true
    config:
      strict_mode: true
      max_size: 50000
```

---

# 25. Global Plugin Config

Example:

```yaml
plugins:
  enabled: true
  directory: ./plugins
  allow_unsigned: false

  better-json:
    enabled: true
    priority: 80
    config:
      strict_mode: true
```

---

# 26. Plugin Security

Plugins are untrusted by default.

Security rules:

1. Deny permissions by default.
2. Require explicit dangerous permission approval.
3. Do not allow unrestricted filesystem access.
4. Do not allow network access unless declared.
5. Do not expose full prompts unless permission allows.
6. Do not expose memory unless permission allows.
7. Log plugin actions in telemetry.
8. Allow users to disable plugins easily.

---

# 27. Plugin Sandboxing

V1 sandboxing can be limited.

Long-term sandboxing target:

```text
WASM sandbox
capability-based permissions
no default network access
limited filesystem access
memory isolation
deterministic host APIs
```

---

# 28. Plugin Signing

V1 may not require signing.

Future plugin signing:

```text
signed plugins
trusted publishers
checksum verification
local allowlist
enterprise policy
```

Config placeholder:

```yaml
plugins:
  allow_unsigned: false
  trusted_publishers:
    - novexa-official
```

---

# 29. Plugin Directories

Default directories:

```text
./plugins
~/.novexa/plugins
```

Example structure:

```text
plugins/
└── better-json/
    ├── novexa.plugin.yaml
    ├── plugin.wasm
    └── README.md
```

---

# 30. Built-In Plugins

Novexa may ship core behaviour as built-in plugins internally.

Examples:

```text
novexa-json-repair
novexa-anti-loop
novexa-markdown-validator
novexa-qwen-profile
novexa-ollama-provider
```

Built-in plugins may be enabled by default.

---

# 31. Plugin Registry Future

Future marketplace structure:

```text
Novexa Registry
├── Plugin metadata
├── Versions
├── Publisher identity
├── Checksums
├── Compatibility
├── Categories
├── Ratings
└── Security status
```

V1 should not require registry access.

---

# 32. Plugin CLI Commands

V1 or future CLI:

```bash
novexa plugin list
novexa plugin show better-json
novexa plugin enable better-json
novexa plugin disable better-json
novexa plugin validate ./plugins/better-json
```

Future:

```bash
novexa plugin install better-json
novexa plugin update better-json
novexa plugin remove better-json
novexa plugin trust publisher-name
```

---

# 33. Plugin Dashboard Features

Dashboard should eventually show:

- installed plugins
- plugin status
- enabled/disabled state
- permissions
- hook usage
- plugin errors
- plugin latency
- plugin telemetry events
- dangerous permission warnings

---

# 34. Plugin Telemetry Events

Required plugin events:

```text
plugin_discovered
plugin_manifest_validated
plugin_loaded
plugin_disabled
plugin_hook_started
plugin_hook_completed
plugin_hook_failed
plugin_permission_denied
plugin_change_applied
plugin_change_rejected
plugin_shutdown
```

Example:

```yaml
event: plugin_hook_completed
engine: plugin
metadata:
  plugin_id: better-json
  hook: after_validation
  latency_ms: 4
  status: success
```

---

# 35. Provider Plugins

Provider adapters may eventually be plugins.

Example plugin:

```yaml
id: novexa-provider-vllm
name: vLLM Provider Adapter
type: provider

permissions:
  - register_provider
  - network_access

hooks:
  - provider_register
```

Provider plugin must implement Provider Adapter contract.

---

# 36. Model Profile Plugins

A model profile pack can be a plugin.

Example:

```yaml
id: qwen-profile-pack
name: Qwen Model Profile Pack
type: model_profile

permissions:
  - register_model_profiles

hooks:
  - profile_register
```

This allows community model tuning packs.

---

# 37. Prompt Plugins

Prompt plugins modify prompt behaviour.

Examples:

```text
better-coding-prompt
strict-json-prompt
malay-language-style
technical-answer-style
```

Prompt plugins may hook into:

```text
before_prompt
after_prompt
```

Permissions:

```text
read_prompt
modify_prompt
emit_telemetry
```

---

# 38. Validation Plugins

Validation plugins add validators.

Examples:

```text
json-schema-plus
markdown-table-validator
citation-format-checker
code-block-validator
```

Hooks:

```text
before_validation
after_validation
```

Permissions:

```text
read_response
read_validation_report
modify_validation_report
emit_telemetry
```

---

# 39. Repair Plugins

Repair plugins add repair strategies.

Examples:

```text
better-json-repair
markdown-cleanup
loop-output-cleaner
```

Hooks:

```text
before_repair
after_validation
```

Permissions:

```text
read_response
modify_response
read_validation_report
emit_telemetry
```

---

# 40. Context Plugins

Context plugins add context processing behaviour.

Examples:

```text
log-compressor
code-context-prioritizer
chat-history-deduper
```

Hooks:

```text
before_context
after_context
```

Permissions:

```text
read_context
modify_context
emit_telemetry
```

---

# 41. Plugin API Stability

Hook names and permission names are public API.

They should change slowly.

Breaking plugin API changes require:

- new plugin schema version
- migration notes
- compatibility layer if possible

---

# 42. Plugin Development Rules

Plugin authors should:

1. Keep plugins focused.
2. Request minimum permissions.
3. Emit useful telemetry.
4. Fail gracefully.
5. Avoid storing prompts unnecessarily.
6. Avoid network access unless essential.
7. Document behaviour clearly.
8. Include tests.
9. Include examples.
10. Avoid hidden side effects.

---

# 43. Plugin Testing Requirements

Plugin Engine tests:

- manifest validation
- invalid manifest rejection
- permission enforcement
- hook registration
- hook execution order
- plugin failure recovery
- required plugin failure
- plugin config loading
- plugin telemetry events
- plugin change application
- plugin change rejection

Plugin author tests:

- plugin initialization
- hook input handling
- hook output validity
- config validation
- permission assumptions
- failure scenarios

---

# 44. V1 Implementation Priority

Implement in this order:

```text
1. Plugin manifest schema
2. Plugin discovery
3. Plugin validation
4. Built-in plugin registry
5. Hook registry
6. Hook execution placeholder
7. Permission model placeholder
8. Plugin telemetry
9. Enable/disable plugins from config
10. Native/internal plugin support
11. Third-party plugin loading
12. WASM support
```

Reason:

Design plugin architecture early, but do not block V1 runtime on full third-party plugin execution.

---

# 45. Anti-Patterns

Avoid:

```text
Plugins bypassing Pipeline Engine
Plugins getting full Pipeline Context by default
Plugins mutating config silently
Plugins calling providers directly
Plugins storing prompt data without permission
Plugins crashing runtime
Plugins with vague permissions
Plugins that do too many things
Plugin system becoming required for basic runtime
Marketplace before runtime is useful
```

---

# 46. Final Plugin Statement

The Plugin System makes Novexa extensible without making the core runtime heavy.

Plugins should feel powerful but controlled.

The best version of Novexa has a small, stable core and a rich ecosystem of optional extensions around it.