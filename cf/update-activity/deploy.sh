gcloud beta functions deploy update-activity --entry-point UpdateActivity --trigger-topic update-activity-topic --set-env-vars CONFIG_BUCKET=football-gobot-config --set-env-vars CONFIG_OBJECT=config.json --set-env-vars PROJECT=football-gobot --runtime go116 --docker-repository=projects/football-gobot/locations/us-central1/repositories/update-activity