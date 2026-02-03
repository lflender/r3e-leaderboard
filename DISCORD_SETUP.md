# Discord Integration Setup

## Security Warning ⚠️
**NEVER commit your Discord bot token to Git!** Anyone with your token can:
- Send messages as your bot
- Read all messages in channels your bot has access to  
- Impersonate your bot completely

The `discord_token.txt` file is git-ignored to prevent accidental commits.

## Setup Steps

### 1. Create a Discord Bot (5 minutes)

1. Go to https://discord.com/developers/applications
2. Click **"New Application"**, give it a name (e.g., "R3E Leaderboard Reader")
3. Go to the **"Bot"** section in the left sidebar
4. Click **"Add Bot"** (or **"Reset Token"** if it already exists)
5. Click **"Copy"** to copy your bot token

### 2. Configure Bot Permissions

1. Still in the Bot section, scroll down to **"Privileged Gateway Intents"**
2. Enable **"Message Content Intent"** (required to read message content)
3. Click **"Save Changes"**

### 3. Invite Bot to Your Server

1. Go to **"OAuth2"** → **"URL Generator"** in the left sidebar
2. Under **Scopes**, select:
   - ✅ `bot`
3. Under **Bot Permissions**, select:
   - ✅ `View Channels`
   - ✅ `Read Message History`
   - ✅ `Read Messages/View Channels`
4. Copy the generated URL at the bottom
5. Open the URL in your browser and select the Discord server to add the bot to
6. Click **"Authorize"**

### 4. Add Token to This Project

1. Copy the example file:
   ```bash
   cp discord_token.txt.example discord_token.txt
   ```

2. Edit `discord_token.txt` and paste your bot token (just the token, nothing else)

3. **Verify it's git-ignored:**
   ```bash
   git status
   ```
   You should NOT see `discord_token.txt` in the list. If you do, **DO NOT COMMIT IT!**

### 5. Test the Integration

Run your application - it will automatically detect the token and enable Discord integration:

```bash
go run .
```

Look for log messages like:
```
📡 Checking Discord for Daily Sprint Races (last 15 minutes)...
📨 Found X messages in the last 5 minutes
🏁 Found Daily Sprint Races message from ...
✅ Parsed X races from Discord message
```

## Configuration

The bot is configured in [internal/config.go](internal/config.go):
- **Channel ID**: `928940507319664660` (R3E schedule channel)
- **Check interval**: Last 5 minutes
- **Auto-enabled**: When `discord_token.txt` exists and has a token

## Troubleshooting

**"Discord API error: 401"**
- Your token is invalid or expired
- Generate a new token in the Discord Developer Portal

**"Discord API error: 403"**  
- Bot doesn't have permission to read the channel
- Make sure the bot is in the server and has "Read Message History" permission

**"No Daily Sprint Races message found"**
- No message with "Daily Sprint Races" was posted in the last 5 minutes
- This is normal - the feature checks periodically

**Token not being read**
- Make sure `discord_token.txt` is in the project root directory (same level as `main.go`)
- Make sure there are no extra spaces or newlines in the file
