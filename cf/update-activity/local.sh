cd cmd && PROJECT=football-gobot FUNCTION_TARGET=UpdateActivity CONFIG_BUCKET=football-gobot-config CONFIG_OBJECT=config.json go run .

# Then call this using:
# curl localhost:8080 -X POST -H "Content-Type: application/json" -d '{}'