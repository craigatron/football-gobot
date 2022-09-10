cd cmd && PROJECT=football-gobot FUNCTION_TARGET=UpdateScores CONFIG_BUCKET=football-gobot-config CONFIG_OBJECT=config.json PROJECTION_BUCKET=projection-bucket go run .

# Then call this using:
# curl localhost:8080 -X POST -H "Content-Type: application/json" -d '{}'