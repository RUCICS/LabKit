import json
import pathlib


def main() -> None:
    submission = pathlib.Path("/submission/submission.json")
    payload = json.loads(submission.read_text(encoding="utf-8"))
    score = float(payload.get("score", 0.0))

    print("labkit local smoke evaluator")
    print(
        json.dumps(
            {
                "verdict": "scored",
                "scores": {"score": score},
                "detail": {
                    "format": "markdown",
                    "content": f"# Local Smoke\n\nReceived score: {score:.2f}",
                },
            },
            separators=(",", ":"),
        )
    )


if __name__ == "__main__":
    main()
