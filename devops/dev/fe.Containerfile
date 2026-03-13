FROM node:25-alpine

WORKDIR /app

# Copy package files (handles missing package-lock.json)
COPY frontend/package.json ./
RUN npm install

# Copy source
COPY frontend/ ./

EXPOSE 5173

CMD ["npm", "run", "dev"]