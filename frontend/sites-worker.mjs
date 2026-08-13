const worker = {
  async fetch(request, env) {
    const url = new URL(request.url)
    let response = await env.ASSETS.fetch(request)

    if (response.status === 404 && !url.pathname.includes('.')) {
      response = await env.ASSETS.fetch(new Request(new URL('/index.html', request.url), request))
    }

    const headers = new Headers(response.headers)
    headers.set('X-Content-Type-Options', 'nosniff')
    headers.set('Referrer-Policy', 'strict-origin-when-cross-origin')
    headers.set('X-Frame-Options', 'DENY')
    return new Response(response.body, { status: response.status, statusText: response.statusText, headers })
  },
}

export default worker
