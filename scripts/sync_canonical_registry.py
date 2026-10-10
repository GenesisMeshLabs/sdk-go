"""Write genesismesh/canonical_registry.json from the shared conformance suite.

genesismesh/testdata/conformance/field_registry.json is a copy of the core's
conformance/vectors/field_registry.json. After copying a new one, run:

    python scripts/sync_canonical_registry.py

The conformance test fails while the embedded registry differs from the suite.
"""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
suite = json.loads((ROOT / "genesismesh" / "testdata" / "conformance" / "field_registry.json").read_text(encoding="utf-8"))
target = ROOT / "genesismesh" / "canonical_registry.json"
target.write_text(json.dumps(suite["registry"], indent=2) + "\n", encoding="utf-8", newline="\n")
print(f"{target.relative_to(ROOT)}: {len(suite['registry']['models'])} models, version {suite['registry']['version']}")
