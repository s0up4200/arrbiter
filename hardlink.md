## Feature Request: Non-Hardlinked Movie Management for Radarr/qBittorrent

### Research Requirements
Use deepwiki MCP to research the following libraries:
- `autobrr/go-qbittorrent` - for qBittorrent API interactions
- `golift/starr` - for Radarr API interactions
- `Radarr/Radarr` (if needed) - for additional API documentation

### Feature Overview
Create a new command (separate from existing delete/list commands) that manages non-hardlinked movies between Radarr and qBittorrent to ensure proper hardlinking.

### Core Functionality

1. **Detection Phase**
   - Scan Radarr library for movies that are NOT hardlinked to any other location on the system
   - Research if hardlink status can be detected via Radarr API, otherwise implement using system calls

2. **Processing Logic**
   For each non-hardlinked movie found:
   
   a. **If movie exists in qBittorrent (seeding)**:
      - Verify the torrent's quality matches the movie's quality profile in Radarr
      - If quality matches: Import the existing torrent file to Radarr
      - Research Radarr's manual import API (available in WebUI: Wanted → Missing → Manual Import)
   
   b. **If movie NOT in qBittorrent**:
      - Prompt user: "Delete current file and search for new version?"
      - If yes: Delete existing file and trigger new search in Radarr
      - This ensures the new download will be properly hardlinked between qBittorrent and Radarr

3. **Quality Profile Validation**
   - When importing existing torrents, validate against Radarr's quality profile
   - Research how to retrieve and compare quality profiles via golift/starr or Radarr API

### UX Considerations
Design an efficient user experience for handling multiple non-hardlinked movies discovered in a single scan. Consider:
- Batch processing options
- Individual movie selection
- Summary view before actions
- Progress indicators

### Technical Implementation Notes
- Prioritize API-based solutions where available
- Fall back to system calls for hardlink detection if necessary
- Ensure proper error handling for API failures
- Maintain transaction safety when deleting/re-downloading