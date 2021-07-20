# Project Radiation

Project Radiation is a CLI client for [miniflux](https://miniflux.app/). 

The function of this program includes listing unread entries, viewing articles and marking viewed article as read.

### Configuration

Radiation reads a configuration file located in `~/.radiation` formatted in JSON. The config file is expected to have these fields:

- "server_url": url of the miniflux server
- "token": API token for authentication
- "page_entries": max number of entries on one page of entry list
- "lines": max number of lines on one page of article
