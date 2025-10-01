import argparse
import json
import textwrap
from pathlib import Path
import io

import torch
from PIL import Image
from torchvision import models, transforms
from torchvision.models import ResNet50_Weights
import requests

def init():
    """Returns the ResNet50 pre-trained model and the output labels."""
    # Hack to have static variables attached to a function.
    if not hasattr(init, "model"):
        model = models.resnet50(weights=ResNet50_Weights.IMAGENET1K_V1)
        model.eval()

        classes_url = "https://raw.githubusercontent.com/pytorch/hub/master/imagenet_classes.txt"
        labels = requests.get(classes_url).text.strip().split("\n")

        init.model = model
        init.labels = labels

    return init.model, init.labels


def classify(pillow_image, threshold=0.1):
    model, labels = init()

    # Load and preprocess image. Requires since ResNet50 is trained on ImageNet
    # and the given image must be adapted to the expected input format as in
    # ImageNet.
    preprocess = transforms.Compose([
        transforms.Resize(256),
        transforms.CenterCrop(224),
        transforms.ToTensor(),
        transforms.Normalize(mean=[0.485, 0.456, 0.406],
                             std=[0.229, 0.224, 0.225]),
    ])
    input_tensor = preprocess(pillow_image)  # Shape [3, 224, 224].

    # Required even if we have only one image (shape [1, 3, 224, 224]).
    input_batch = input_tensor.unsqueeze(0)
    
    with torch.no_grad():
        output = model(input_batch)
    
    # Apply softmax to get probabilities.
    probabilities = torch.nn.functional.softmax(output[0], dim=0)

    # Get top 3 predictions above threshold.
    predictions = []
    for i in torch.argsort(probabilities, descending=True):
        prob = probabilities[i].item()
        if prob >= threshold:
            predictions.append({"class": labels[i], "probability": prob})

        if len(predictions) >= 3:  # Only take top 3
            break
   
    return predictions

def handle(event, context):
    """Function executed by OpenFaaS when a request is incoming."""
    if event.method != "POST":
        return {
            "statusCode": 405,
            "body": "Method not allowed"
        }

    match event.headers.get("Content-Type"):
        case "image/jpeg" | "image/png":
            try:
                input_image = Image.open(io.BytesIO(event.body))
            except:
                return {
                    "statusCode": 400,
                    "body": "Bad image."
                }
        case _:
            return {
                "statusCode": 400,
                "body": "Unsupported content type."
            }

    results = classify(input_image)

    return {
        "statusCode": 200,
        "body": json.dumps(results),
        "headers": {
            "Content-Type": "application/json"
        }
    }


def main():
    """Main function run only if this Pythom module is executed from command
    line."""
    desc = textwrap.dedent("""Classify an image using a pre-trained model (ResNet50) and output
    a JSON with the top 3 labels.""")

    parser = argparse.ArgumentParser(description=desc)
    parser.add_argument("image_path", type=Path, help="Path to the input image")
    parser.add_argument("--threshold",
                        type=float,
                        default=0.1,
                        help="Skip labels under the given threshold (default: 0.1)")
    args = parser.parse_args()
    
    input_image = Image.open(image_path)

    results = classify(input_image, args.threshold)
    
    print(json.dumps(results, indent=2))

if __name__ == "__main__":
    main()
