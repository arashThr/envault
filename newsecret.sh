SECRETS_BASE="$HOME/Documents/workspace/secrets"

newsecret() {
  if [ $# -ne 2 ]; then
    echo "Usage: newsecret <project-name> <filename>"
    return 1
  fi

  PROJECT="$1"
  FILENAME="$2"

  PROJECT_DIR="$SECRETS_BASE/$PROJECT"
  TARGET_FILE="$PROJECT_DIR/$FILENAME"
  LINK_PATH="$(pwd)/$FILENAME"

  # Ensure project secrets directory exists
  mkdir -p "$PROJECT_DIR"

  # Prevent overwriting existing files
  if [ -e "$TARGET_FILE" ]; then
    echo "Error: secret already exists: $TARGET_FILE"
    return 1
  fi

  if [ -e "$LINK_PATH" ]; then
    echo "Error: file already exists in current directory: $LINK_PATH"
    return 1
  fi

  # Create the secret file with strict permissions
  umask 077
  touch "$TARGET_FILE"

  # Create symlink in current directory
  ln -s "$TARGET_FILE" "$LINK_PATH"

  echo "Secret created successfully:"
  echo "  Project : $PROJECT"
  echo "  File    : $TARGET_FILE"
  echo "  Symlink : $LINK_PATH"
}

encrypt_secrets() {
  OUTPUT_FILE="$HOME/Documents/secrets.age"
  RECIPIENTS_URL="https://github.com/arashthr.keys"

  if [ ! -d "$SECRETS_BASE" ]; then
    echo "Secrets directory not found: $SECRETS_BASE"
    return 1
  fi

  echo "Fetching public keys from GitHub…"

  RECIPIENTS=$(curl -fsSL "$RECIPIENTS_URL")
  if [ -z "$RECIPIENTS" ]; then
    echo "Failed to fetch public keys."
    return 1
  fi

  echo "Encrypting secrets directory…"

  tar -czf - -C "$(dirname "$SECRETS_BASE")" "$(basename "$SECRETS_BASE")" \
    | age $(printf -- "-R <(printf '%s\n' \"%s\") " "$RECIPIENTS") \
    -o "$OUTPUT_FILE"

  if [ $? -eq 0 ]; then
    echo "Encrypted successfully:"
    echo "  $OUTPUT_FILE"
  else
    echo "Encryption failed."
    return 1
  fi
}
