#!/bin/sh

gcloud functions deploy flashlight \
	--gen2 \
	--region=northamerica-northeast2 \
	--runtime=go121 \
	--entry-point=flashlight \
	--trigger-http \
	--max-instances=1 \
	--min-instances=0 \
	--timeout=30s \
	--cpu=1 \
	--memory=128Mi \
	--allow-unauthenticated \
	--concurrency 100 \
	--set-secrets HYPIXEL_API_KEY=prism-hypixel-api-key:latest
