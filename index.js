import { env } from "process"
import crypto from "crypto"
import got from "got"
import OAuth from "oauth-1.0a"

const interval = 60 * 1000 // 60 seconds

const oauth = OAuth({
  consumer: {
    key: env.TWITTER_CONSUMER_KEY,
    secret: env.TWITTER_CONSUMER_SECRET,
  },
  signature_method: "HMAC-SHA1",
  hash_function: (baseString, key) =>
    crypto.createHmac("sha1", key).update(baseString).digest("base64"),
})

async function sunApi(date = "today") {
  const {
    results: { sunrise, sunset },
  } = await got(
    `https://api.sunrise-sunset.org/json?lat=13.7563&lng=100.5018&date=${date}&formatted=0`
  ).json()
  return { sunrise: new Date(sunrise), sunset: new Date(sunset) }
}

async function unsplashApi() {
  const { urls } = await got(`https://api.unsplash.com/photos/random`, {
    headers: {
      "Accept-Version": "v1",
      Authorization: `Client-ID ${env.UNSPLASH_ACCESS_KEY}`,
    },
  }).json()
  return urls.raw
}

async function nasaApi() {
  const [image] = await got(
    `https://api.nasa.gov/planetary/apod?api_key=DEMO_KEY&count=1`
  ).json()
  if (image.media_type !== "image") return nasaApi()
  return image.hdurl || image.url
}

async function downloadImage(url) {
  return (await got(url).buffer()).toString("base64")
}

async function twitterApi(base64Img) {
  const bannerUrl = `https://api.twitter.com/1.1/account/update_profile_banner.json`
  const authHeader = oauth.toHeader(
    oauth.authorize(
      {
        url: bannerUrl,
        method: "POST",
        data: {
          banner: base64Img,
        },
      },
      {
        key: env.TWITTER_ACCESS_TOKEN,
        secret: env.TWITTER_ACCESS_TOKEN_SECRET,
      }
    )
  )
  await got.post(bannerUrl, {
    headers: {
      Authorization: authHeader["Authorization"],
    },
    form: {
      banner: base64Img,
    },
  })
}

async function main() {
  const { sunrise, sunset } = await sunApi()

  const isSunrise = Math.abs(sunrise.getTime() - Date.now()) < interval / 2
  const isSunset = Math.abs(sunset.getTime() - Date.now()) < interval / 2

  if (isSunrise || isSunset) {
    const url = await (isSunrise ? unsplashApi() : nasaApi())
    const base64Img = await downloadImage(url)
    twitterApi(base64Img)
    console.log(`${Date()}:${url}`)
  }
}

main()
setInterval(main, interval)
