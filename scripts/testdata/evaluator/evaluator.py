import json
import pathlib


def main() -> None:
    submission = pathlib.Path("/submission/e2e-submission.json")
    print("labkit e2e evaluator")
    payload = json.loads(submission.read_text(encoding="utf-8"))
    print(json.dumps(payload, separators=(",", ":")))


if __name__ == "__main__":
    main()
