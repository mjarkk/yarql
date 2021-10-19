# Relay example

# Backend

```sh
cd backend
go run .
```

# Frontend

```sh
cd frontend
npm install

# Generate the schema.graphql file
# Note backend needs to be running for this step
npm run get-schema

# Run the relay compiler
npm run relay

# Start the frontend
# Note to view the website the backend needs to be running
npm run start
```
