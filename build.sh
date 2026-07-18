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
        # Verify gcloud is authenticated
        GCLOUD_ACCOUNT=$(gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null)
        if [ -z "$GCLOUD_ACCOUNT" ]; then
            echo "ERROR: Not authenticated with gcloud. Run: gcloud auth login"
            exit 1
        fi
        echo "Authenticated as: $GCLOUD_ACCOUNT"

        # Configure project and log Docker into the registry
        gcloud config set project "$PROJECT_ID" --quiet
        gcloud auth print-access-token | docker login -u oauth2accesstoken --password-stdin us.gcr.io

        source .env
        if [ -n "$RESEND_API_KEY" ]; then
           echo "Resend API key is set – ready to deploy"
        else
            echo "ERROR: RESEND_API_KEY is missing or empty"
            exit 1
        fi

        if [ -n "$CLOUDFLARE_SECRET_KEY" ]; then
            echo "Cloudflare Secret Key is set"
        else 
            echo "ERROR:  CLOUDFLARE_SECRET_KEY is missing or empty"
            exit 1
        fi

        if [ -n "$GOOGLE_OAUTH_SECRET" ]; then
            echo "Google Oauth Client Secret is set"
        else
            echo "ERROR:  GOOGLE_OAUTH_SECRET is missing or empty"
            exit 1
        fi

        if [ -n "$ANTHROPIC_API_KEY" ]; then
            echo "Anthropic API key is set"
        else
            echo "ERROR: ANTHROPIC_API_KEY is missing or empty"
            exit 1
        fi
        
        echo "Building Docker image..."
        docker build -t "$IMAGE_NAME:latest" . || { echo "Error: Docker build failed"; exit 1; }
        echo "Tagging image for GCR..."
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
            --set-env-vars RESEND_API_KEY=${RESEND_API_KEY} \
            --set-env-vars CLOUDFLARE_SECRET_KEY=${CLOUDFLARE_SECRET_KEY} \
            --set-env-vars GOOGLE_OAUTH_SECRET=${GOOGLE_OAUTH_SECRET} \
            --set-env-vars DATABASE_URL=${DATABASE_URL_PROD} \
            --set-env-vars SESSION_SECRET=${SESSION_SECRET} \
            --set-env-vars ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
        echo "Deployed to Cloud Run!"
        ;;
    *)
        echo "Running tests..."
        go test ./... || { echo "Tests failed — aborting build"; exit 1; }
        echo "Building local binary..."
        go build -o colout2 .
        echo "Local binary built: ./colout2"
        ;;
esac
