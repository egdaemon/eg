### generate missing golang tests using a local LLM

Runs the existing tests, records coverage, then asks the model to generate tests for the least covered functions.

Requires 32GiB of VRAM for the model.

```bash
eg compute local --vram 32GiB dev/egautogen
```
