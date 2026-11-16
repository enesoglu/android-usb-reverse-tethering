"""Request graceful shutdown of this project's current USB VPN controller."""
import json
from pathlib import Path


def main():
    metadata=Path(".local/controller.json")
    if not metadata.is_file():
        print("No running project controller recorded")
        return
    pid=json.loads(metadata.read_text(encoding="utf-8")).get("pid")
    if not isinstance(pid,int) or pid<=0: raise RuntimeError("invalid controller metadata")
    (metadata.parent / f"stop-{pid}").write_text("stop\n",encoding="utf-8")
    print("Disconnect requested")


if __name__=="__main__": main()
