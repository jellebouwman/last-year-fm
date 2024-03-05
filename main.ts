import { URLSearchParams } from "url"

const unixStartOf2023 = 1672527600
const unixStartOf2024 = 1704063600
const lastFmApiRoot = 'http://ws.audioscrobbler.com/2.0/'
const username = 'jellebouwman'
const apiKey = '36f4da5da9b7053e06f5d875a64d0129'

async function main() {
  const params = new URLSearchParams()

  params.set('api_key', apiKey)
  params.set('user', username)
  params.set('limit', '200')
  params.set('from', unixStartOf2023.toString())
  params.set('to', unixStartOf2024.toString())
  params.set('method', 'user.getRecentTracks')
  params.set('format', 'json')

  const url = new URL(lastFmApiRoot + '?' + params.toString())

  const res = await fetch(url)
  const body = await res.json()
  if (body && body.recenttracks) {
    console.log('Found', body.recenttracks['@attr'].total, 'tracks in 2023')
  }
}

// {
//   "artist": {
//     "mbid": "a16371b9-7d36-497a-a9d4-42b0a0440c5e",
//     "#text": "Slowdive"
//   },
//   "streamable": "0",
//   "image": [ ],
//   "mbid": "729400f2-60e8-4eda-b1e7-538cdaee7743",
//   "album": {
//       "mbid": "4acdaa51-aa44-4a9b-954f-3c6eaab65590",
//       "#text": "everything is alive"
//   },
//   "name": "chained to a cloud",
//   "url": "https://www.last.fm/music/Slowdive/_/chained+to+a+cloud",
//   "date": {
//       "uts": "1703951089",
//       "#text": "30 Dec 2023, 15:44"
//   }
// },
// {
//   "artist": {
//     "mbid": "a16371b9-7d36-497a-a9d4-42b0a0440c5e",
//     "#text": "Slowdive"
//   },
//   "streamable": "0",
//   "image": [ ],
//   "mbid": "729400f2-60e8-4eda-b1e7-538cdaee7743",
//   "album": {
//       "mbid": "4acdaa51-aa44-4a9b-954f-3c6eaab65590",
//       "#text": "everything is alive"
//   },
//   "name": "chained to a cloud",
//   "url": "https://www.last.fm/music/Slowdive/_/chained+to+a+cloud",
//   "date": {
//       "uts": "17039554782",
//       "#text": "16 Nov 2023, 13:44"
//   }
// },


main().then(() => console.log('Exit without errors!')).catch((e) => console.error(e))