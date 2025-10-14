#!/bin/bash

# Build script for colout2
# Usage: ./build.sh [docker|deploy]

PROJECT_ID="columbia-outdoor"  # Your GCP project ID
IMAGE_NAME="colout2"

### Change from Container Registry to Artifact Registry 
##GCR_IMAGE="gcr.io/$PROJECT_ID/$IMAGE_NAME:latest"
GCR_IMAGE="us.gcr.io/$PROJECT_ID/$IMAGE_NAME:latest"

case "$1" in
    "docker")
        echo "Building Docker image..."
        docker build -t "$IMAGE_NAME:latest" .
        echo "Docker image built: $IMAGE_NAME:latest"
        echo "Run locally:  docker run -p 8080:8080 colout2:latest"
        ;;
    "deploy")
        ## Use the latest build
        echo "Tagging last built image for GCR..."
        docker tag "$IMAGE_NAME:latest" "$GCR_IMAGE"
        echo "Pushing to GCR..."
        docker push "$GCR_IMAGE"
        echo "Deploying to Cloud Run..."
        gcloud run deploy "$IMAGE_NAME" \
            --image "$GCR_IMAGE" \
            --platform managed \
            --region us-central1 \
            --port 8080 \
            --allow-unauthenticated
        echo "Deployed to Cloud Run!"
        ;;
    *)
        echo "Building local binary..."
        go build -o colout2 .
        echo "Local binary built: ./colout2"
        ;;
esac