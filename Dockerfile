FROM node:20-alpine

# Install Claude CLI
RUN npm install -g @anthropic-ai/claude-code

# Set working directory
WORKDIR /app

# Set environment variables (can be overridden)
ENV CLAUDE_CODE_OAUTH_TOKEN=""

# Entry point for running Claude
ENTRYPOINT ["claude"]
