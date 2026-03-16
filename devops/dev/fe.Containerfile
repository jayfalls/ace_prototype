FROM node:25-alpine

WORKDIR /app

# Copy only package files (for caching)
COPY frontend/package.json ./
COPY frontend/package-lock.json ./

# Install dependencies (in container, not on host)
RUN npm install

# Copy source
COPY frontend/ ./

EXPOSE 5173

CMD ["npm", "run", "dev"]
