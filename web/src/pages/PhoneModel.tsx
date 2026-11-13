import { useEffect, useRef, useState } from 'react'
import * as THREE from 'three'
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js'
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js'

// PhoneModel: a real 3D phone, with Cue's phone clip playing on its screen.
// It turns gently toward the pointer and floats a little. Until the model
// has loaded (or where there's no WebGL, or the viewer asks for less
// motion) the drawn phone underneath shows instead. It draws only while
// it's on screen.
export default function PhoneModel({ model, clip, poster }: { model: string; clip: string; poster: string }) {
  const box = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const el = box.current
    if (!el || matchMedia('(prefers-reduced-motion: reduce)').matches) return
    let renderer: THREE.WebGLRenderer
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: 'low-power' })
    } catch {
      return // no WebGL: the drawn phone stays
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    renderer.outputColorSpace = THREE.SRGBColorSpace
    renderer.toneMapping = THREE.ACESFilmicToneMapping
    renderer.toneMappingExposure = 1.05
    el.appendChild(renderer.domElement)

    const scene = new THREE.Scene()
    const pmrem = new THREE.PMREMGenerator(renderer)
    const env = pmrem.fromScene(new RoomEnvironment(), 0.04).texture
    scene.environment = env
    const camera = new THREE.PerspectiveCamera(22, 1, 0.01, 100)

    // The clip, as the phone's screen.
    const video = document.createElement('video')
    Object.assign(video, { src: clip, poster, muted: true, loop: true, playsInline: true, crossOrigin: 'anonymous' })
    video.setAttribute('playsinline', '')
    const screen = new THREE.VideoTexture(video)
    screen.colorSpace = THREE.SRGBColorSpace
    screen.flipY = false

    const rig = new THREE.Group()
    scene.add(rig)
    let stopped = false
    new GLTFLoader().load(
      model,
      (gltf) => {
        if (stopped) return
        const phone = gltf.scene
        phone.traverse((o) => {
          const mesh = o as THREE.Mesh
          if (mesh.isMesh && mesh.name === 'Front_Screen') {
            mesh.material = new THREE.MeshBasicMaterial({ map: screen, toneMapped: false })
          }
        })
        // Centred, standing up, sized to fill the view.
        const size = new THREE.Box3().setFromObject(phone)
        const centre = size.getCenter(new THREE.Vector3())
        const dims = size.getSize(new THREE.Vector3())
        phone.position.sub(centre)
        rig.add(phone)
        const height = Math.max(dims.x, dims.y, dims.z)
        camera.position.set(0, 0, (height / 2 / Math.tan(THREE.MathUtils.degToRad(camera.fov / 2))) * 1.08)
        camera.lookAt(0, 0, 0)
        resize()
        setReady(true)
      },
      undefined,
      () => undefined, // couldn't load: the drawn phone stays
    )

    function resize() {
      const w = el!.clientWidth
      const h = el!.clientHeight
      if (!w || !h) return
      renderer.setSize(w, h, false)
      camera.aspect = w / h
      camera.updateProjectionMatrix()
    }
    const ro = new ResizeObserver(resize)
    ro.observe(el)

    // Turns toward the pointer, eased; floats when left alone.
    const aim = { x: 0, y: 0 }
    const now = { x: 0, y: 0 }
    const onMove = (e: PointerEvent) => {
      aim.y = (e.clientX / window.innerWidth - 0.5) * 0.5
      aim.x = (e.clientY / window.innerHeight - 0.5) * 0.25
    }
    window.addEventListener('pointermove', onMove, { passive: true })

    let frame = 0
    let visible = false
    const t0 = performance.now()
    const draw = () => {
      frame = requestAnimationFrame(draw)
      const t = (performance.now() - t0) / 1000
      now.x += (aim.x - now.x) * 0.06
      now.y += (aim.y - now.y) * 0.06
      rig.rotation.set(now.x + Math.sin(t * 0.6) * 0.02, -0.28 + now.y + Math.sin(t * 0.4) * 0.05, 0.05)
      rig.position.y = Math.sin(t * 0.8) * 0.002
      renderer.render(scene, camera)
    }
    const io = new IntersectionObserver(([e]) => {
      visible = e.isIntersecting
      if (visible && !frame) {
        void video.play().catch(() => undefined)
        draw()
      } else if (!visible && frame) {
        cancelAnimationFrame(frame)
        frame = 0
        video.pause()
      }
    })
    io.observe(el)

    return () => {
      stopped = true
      io.disconnect()
      ro.disconnect()
      window.removeEventListener('pointermove', onMove)
      cancelAnimationFrame(frame)
      video.pause()
      video.removeAttribute('src')
      screen.dispose()
      env.dispose()
      pmrem.dispose()
      renderer.dispose()
      renderer.domElement.remove()
    }
  }, [model, clip, poster])

  return <div className={`lp-phone3d${ready ? ' on' : ''}`} ref={box} aria-hidden="true" />
}
