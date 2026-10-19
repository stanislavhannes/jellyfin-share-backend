#!/usr/bin/env bash
# Local demo: builds the app from this checkout and runs it next to a throwaway
# Jellyfin with generated test media, then hands out a few sample links.
#
#   scripts/local-demo.sh up      set up what is missing and start everything
#   scripts/local-demo.sh down    stop the containers, keep the data
#   scripts/local-demo.sh reset   stop and delete everything the demo created
#
# Needs Docker, ffmpeg, curl and jq on the host. Everything it creates lives in
# .local/ and .env.local, both ignored by git.

set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE=(docker compose -f docker-compose.local.yml)
JF=http://localhost:8096
APP=http://localhost:8097
JF_USER=demo
JF_PASSWORD=demo
BACKEND_KEY=demo-admin-key
MEDIA=.local/media
CLIENT='MediaBrowser Client="jfshare-demo", Device="script", DeviceId="jfshare-demo", Version="1.0"'

say() { printf '\033[1m==>\033[0m %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null || die "$1 is required"; }

wait_for() { # url, what
  for _ in $(seq 1 120); do
    curl -fs "$1" >/dev/null 2>&1 && return 0
    sleep 2
  done
  die "$2 did not come up at $1"
}

# ---- Test media --------------------------------------------------------------
# Synthetic picture and tone, but shaped like a real library: a film with two
# audio languages and a text subtitle, and a season of three episodes, each
# with local artwork and an NFO so Jellyfin has metadata without the internet.

art() { # out, width, height, filter (with or without options of its own)
  local sep='='
  [[ $4 == *=* ]] && sep=':'
  ffmpeg -loglevel error -y -f lavfi -i "$4${sep}s=$2x$3" -frames:v 1 "$1"
}

clip() { # out, seconds, video source, extra args...
  local out=$1 secs=$2 src=$3; shift 3
  ffmpeg -loglevel error -y \
    -f lavfi -i "$src=size=1280x720:rate=24:duration=$secs" \
    -f lavfi -i "sine=frequency=440:duration=$secs" \
    "$@" -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -ac 2 "$out"
}

make_media() {
  local film="$MEDIA/movies/The Long Walk Home (2024)"
  local show="$MEDIA/shows/Harbour Lights"
  if [ -f "$film/The Long Walk Home (2024).mkv" ] && [ -d "$show/Season 01" ]; then
    return
  fi
  say "Generating test media in $MEDIA"
  mkdir -p "$film" "$show/Season 01"

  cat > "$film/subs.en.srt" <<'SRT'
1
00:00:02,000 --> 00:00:06,000
This is a test film for Jellyfin Share.

2
00:00:08,000 --> 00:00:12,000
The subtitles arrive as a WebVTT sidecar.
SRT
  ffmpeg -loglevel error -y \
    -f lavfi -i "testsrc2=size=1280x720:rate=24:duration=150" \
    -f lavfi -i "sine=frequency=440:duration=150" \
    -f lavfi -i "sine=frequency=660:duration=150" \
    -i "$film/subs.en.srt" \
    -map 0 -map 1 -map 2 -map 3 \
    -c:v libx264 -preset veryfast -pix_fmt yuv420p -c:a aac -ac 2 -c:s srt \
    -metadata:s:a:0 language=eng -metadata:s:a:1 language=ger -metadata:s:s:0 language=eng \
    "$film/The Long Walk Home (2024).mkv"
  rm "$film/subs.en.srt"
  art "$film/poster.jpg" 600 900 "mandelbrot"
  art "$film/backdrop.jpg" 1920 1080 "mandelbrot=start_scale=0.6"
  cat > "$film/movie.nfo" <<'NFO'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
  <title>The Long Walk Home</title>
  <year>2024</year>
  <tagline>Some roads only go one way.</tagline>
  <plot>Test media for Jellyfin Share. After twenty years away, a retired cartographer returns to the coastal town she mapped as a girl, only to find the streets no longer match any map she ever drew.</plot>
  <mpaa>PG-13</mpaa>
  <rating>7.4</rating>
  <genre>Drama</genre>
  <genre>Mystery</genre>
  <studio>Northlight Pictures</studio>
  <director>Ines Varga</director>
  <actor><name>Marta Okafor</name><role>Helen</role></actor>
  <actor><name>Tomás Reyes</name><role>Young Helen</role></actor>
  <actor><name>Ada Lindqvist</name><role>The Postmistress</role></actor>
</movie>
NFO

  art "$show/poster.jpg" 600 900 "cellauto=rule=110"
  art "$show/backdrop.jpg" 1920 1080 "mandelbrot=start_scale=1.2"
  cat > "$show/tvshow.nfo" <<'NFO'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<tvshow>
  <title>Harbour Lights</title>
  <year>2024</year>
  <plot>Test media for Jellyfin Share. A night ferry crew keeps the harbour running while the town sleeps.</plot>
  <genre>Drama</genre>
</tvshow>
NFO
  local names=("Low Tide" "The Night Ferry" "Fog Signal")
  for i in 1 2 3; do
    local ep="$show/Season 01/Harbour Lights S01E0$i"
    clip "$ep.mkv" 60 testsrc -metadata:s:a:0 language=eng
    cat > "$ep.nfo" <<NFO
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<episodedetails>
  <title>${names[$((i - 1))]}</title>
  <season>1</season>
  <episode>$i</episode>
  <plot>Test episode $i of Harbour Lights.</plot>
</episodedetails>
NFO
  done
}

# ---- Jellyfin ----------------------------------------------------------------

jf() { # method, path, [json body] - authenticated once TOKEN is set
  local auth="$CLIENT"
  [ -n "${TOKEN:-}" ] && auth="$CLIENT, Token=\"$TOKEN\""
  curl -fsS -X "$1" "$JF$2" -H "Authorization: $auth" -H 'Content-Type: application/json' \
    ${3:+--data "$3"}
}

setup_jellyfin() {
  say "Starting Jellyfin"
  "${COMPOSE[@]}" up -d jellyfin
  # /health answers while Jellyfin is still migrating; the API is the real signal.
  wait_for "$JF/System/Info/Public" Jellyfin

  if [ "$(jf GET /System/Info/Public | jq -r .StartupWizardCompleted)" != "true" ]; then
    say "Running Jellyfin's setup wizard (user: $JF_USER / $JF_PASSWORD)"
    jf POST /Startup/Configuration '{"UICulture":"en-US","MetadataCountryCode":"DE","PreferredMetadataLanguage":"en"}'
    jf GET /Startup/User >/dev/null
    jf POST /Startup/User "{\"Name\":\"$JF_USER\",\"Password\":\"$JF_PASSWORD\"}"
    jf POST /Startup/RemoteAccess '{"EnableRemoteAccess":true,"EnableAutomaticPortMapping":false}'
    jf POST /Startup/Complete
  fi

  local login
  login=$(jf POST /Users/AuthenticateByName "{\"Username\":\"$JF_USER\",\"Pw\":\"$JF_PASSWORD\"}")
  TOKEN=$(jq -r .AccessToken <<<"$login")
  JF_USER_ID=$(jq -r .User.Id <<<"$login")

  # Offline libraries: the NFOs and local artwork are the metadata, and nothing
  # is looked up online under these made-up titles.
  local options='{"LibraryOptions":{"EnableRealtimeMonitor":false,"TypeOptions":[
    {"Type":"Movie","MetadataFetchers":[],"ImageFetchers":[]},
    {"Type":"Series","MetadataFetchers":[],"ImageFetchers":[]},
    {"Type":"Season","MetadataFetchers":[],"ImageFetchers":[]},
    {"Type":"Episode","MetadataFetchers":[],"ImageFetchers":["Screen Grabber"]}]}}'
  local folders
  folders=$(jf GET /Library/VirtualFolders | jq -r '.[].Name')
  if ! grep -qx Movies <<<"$folders"; then
    say "Adding the Movies library"
    jf POST "/Library/VirtualFolders?name=Movies&collectionType=movies&paths=%2Fmedia%2Fmovies&refreshLibrary=false" "$options"
  fi
  if ! grep -qx Shows <<<"$folders"; then
    say "Adding the Shows library"
    jf POST "/Library/VirtualFolders?name=Shows&collectionType=tvshows&paths=%2Fmedia%2Fshows&refreshLibrary=false" "$options"
  fi
  jf POST /Library/Refresh >/dev/null

  say "Waiting for Jellyfin to scan the test media"
  for _ in $(seq 1 90); do
    MOVIE_ID=$(jf GET "/Users/$JF_USER_ID/Items?Recursive=true&IncludeItemTypes=Movie" | jq -r '.Items[0].Id // empty')
    SEASON_ID=$(jf GET "/Users/$JF_USER_ID/Items?Recursive=true&IncludeItemTypes=Season" | jq -r '.Items[0].Id // empty')
    EPISODES=$(jf GET "/Users/$JF_USER_ID/Items?Recursive=true&IncludeItemTypes=Episode" | jq -r '.TotalRecordCount')
    [ -n "$MOVIE_ID" ] && [ -n "$SEASON_ID" ] && [ "$EPISODES" -ge 3 ] && break
    sleep 2
  done
  [ -n "$MOVIE_ID" ] && [ -n "$SEASON_ID" ] || die "Jellyfin did not find the test media"

  if [ -f .env.local ] && grep -q '^JFSHARE_JELLYFIN_API_KEY=.' .env.local; then
    return
  fi
  say "Creating a Jellyfin API key for the app"
  jf POST "/Auth/Keys?app=jfshare-demo" >/dev/null
  local key
  key=$(jf GET /Auth/Keys | jq -r '[.Items[] | select(.AppName == "jfshare-demo")][0].AccessToken')
  [ -n "$key" ] && [ "$key" != null ] || die "could not read the new API key"
  cat > .env.local <<ENV
# Written by scripts/local-demo.sh for the local demo only.
JFSHARE_JELLYFIN_API_KEY=$key
JFSHARE_BACKEND_API_KEY=$BACKEND_KEY
ENV
}

# ---- The app -----------------------------------------------------------------

admin() { # method, path, [json body]
  curl -fsS -X "$1" "$APP$2" -H "X-Backend-Key: $BACKEND_KEY" -H 'Content-Type: application/json' \
    ${3:+--data "$3"}
}

share() { # label, json body
  local url
  url=$(admin POST /api/admin/shares "$2" | jq -r .publicUrl)
  printf '  %-34s %s\n' "$1" "$url"
}

start_app() {
  say "Building and starting Jellyfin Share (first build takes a few minutes)"
  "${COMPOSE[@]}" up -d --build postgres jfshare
  wait_for "$APP/health" "Jellyfin Share"

  if [ "$(admin GET /api/admin/shares | jq 'if type == "array" then length else (.shares // []) | length end')" -gt 0 ]; then
    say "Sample links already exist - see them at $APP/admin"
    return
  fi
  say "Sample links"
  local base="\"jellyfinUserId\":\"$JF_USER_ID\""
  share "Film, 5 plays, 2 viewers" "{$base,\"jellyfinItemId\":\"$MOVIE_ID\",\"expiresInMinutes\":4320,\"maxTotalPlays\":5,\"maxConcurrentViewers\":2}"
  share "Film, a single play" "{$base,\"jellyfinItemId\":\"$MOVIE_ID\",\"expiresInMinutes\":180,\"maxTotalPlays\":1}"
  share "Film, password: harbour" "{$base,\"jellyfinItemId\":\"$MOVIE_ID\",\"expiresInMinutes\":1440,\"password\":\"harbour\"}"
  share "Season, 3 plays" "{$base,\"jellyfinItemId\":\"$SEASON_ID\",\"expiresInMinutes\":10080,\"maxTotalPlays\":3}"
}

case "${1:-up}" in
  up)
    need docker; need ffmpeg; need curl; need jq
    mkdir -p .local/jellyfin/config .local/jellyfin/cache
    make_media
    setup_jellyfin
    start_app
    echo
    say "Ready"
    echo "  Admin:     $APP/admin   (key: $BACKEND_KEY)"
    echo "  Jellyfin:  $JF          (user: $JF_USER / $JF_PASSWORD)"
    echo "  Stop with: make demo-down"
    ;;
  down)
    "${COMPOSE[@]}" down
    ;;
  reset)
    "${COMPOSE[@]}" down -v
    rm -rf .local .env.local
    ;;
  *)
    die "usage: $0 up|down|reset"
    ;;
esac
