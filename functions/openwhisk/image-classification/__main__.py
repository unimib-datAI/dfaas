# Entry point of the OpenWhisk Python action. The runtime calls main(args) with
# the Web Action parameters and expects a dictionary with statusCode, headers
# and body as response.
#
# See: https://github.com/apache/openwhisk/blob/master/docs/actions-python.md
# See: https://github.com/apache/openwhisk/blob/master/docs/webactions.md
import base64
import io
import json

from PIL import Image

from function import classify


def main(args):
    if args.get("__ow_method") != "post":
        return {"statusCode": 405, "body": "Method not allowed"}

    # OpenWhisk gives header names in lowercase and the body of binary content
    # types as a base64 encoded string, even if the client passed it as binary.
    match args.get("__ow_headers", {}).get("content-type"):
        case "image/jpeg" | "image/png":
            try:
                body = base64.b64decode(args.get("__ow_body", ""))
                input_image = Image.open(io.BytesIO(body))
            except:
                return {"statusCode": 400, "body": "Bad image."}
        case _:
            return {"statusCode": 400, "body": "Unsupported content type."}

    results = classify(input_image)

    return {
        "statusCode": 200,
        "body": json.dumps(results),
        "headers": {"Content-Type": "application/json"},
    }
