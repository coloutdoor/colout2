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
        ## Deploy to GCP.  Run these commands first to get GCP / gcloud working
        ##                   gcloud auth login
        ##                   gcloud config set project columbia-outdoor
        ##                   gcloud auth configure-docker us.gcr.io --quiet
        ## 
        source .env
        if [ -n "$SENDGRID_API_KEY" ]; then
           echo "SendGrid API key is set – ready to deploy"
        else
            echo "ERROR: SENDGRID_API_KEY is missing or empty"
            exit 1
        fi

        echo "Tagging last built image for GCR..."
        docker tag "$IMAGE_NAME:latest" "$GCR_IMAGE"
        echo "Pushing to GCR..."
        docker push "$GCR_IMAGE" || { echo "Error: **** Docker Push failed for $GCR_IMAGE" >&2; exit 1; }
        echo "Deploying to Cloud Run..."
        gcloud run deploy "$IMAGE_NAME" \
            --image "$GCR_IMAGE" \
            --platform managed \
            --region us-central1 \
            --port 8080 \
            --project $PROJECT_ID \
            --allow-unauthenticated \
            --set-env-vars SENDGRID_API_KEY=${SENDGRID_API_KEY}
        echo "Deployed to Cloud Run!"
        ;;
    *)
        echo "Building local binary..."
        go build -o colout2 .
        echo "Local binary built: ./colout2"
        ;;
esac