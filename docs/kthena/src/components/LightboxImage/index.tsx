import React, { useState } from 'react';
import Lightbox from 'yet-another-react-lightbox';
import Zoom from 'yet-another-react-lightbox/plugins/zoom';
import 'yet-another-react-lightbox/styles.css';
import { useColorMode } from '@docusaurus/theme-common';

interface LightboxImageProps {
  src:
    | string
    | { default: string }
    | React.ComponentType<React.SVGProps<SVGSVGElement>>;
  alt?: string;
  title?: string;
  className?: string;
}

const LightboxImage: React.FC<LightboxImageProps> = ({
  src,
  alt,
  title,
  className,
}) => {
  const [open, setOpen] = useState(false);
  const [imageSrc, setImageSrc] = useState<string>('');
  const { colorMode } = useColorMode();
  const backdropColor =
    colorMode === 'dark' ? 'rgba(0, 0, 0, 0.8)' : 'rgba(255, 255, 255, 0.8)';

  // Handle different src types
  React.useEffect(() => {
    if (typeof src === 'string') {
      setImageSrc(src);
    } else if (typeof src === 'function') {
      // SVG imported as React component - convert to data URL
      const svgContainer = document.createElement('div');
      document.body.appendChild(svgContainer);
      const root = require('react-dom/client').createRoot(svgContainer);
      root.render(React.createElement(src, {}));

      setTimeout(() => {
        const svgElement = svgContainer.querySelector('svg');
        if (svgElement) {
          const svgString = new XMLSerializer().serializeToString(svgElement);
          const base64 = btoa(
            String.fromCharCode(...new TextEncoder().encode(svgString)),
          );
          const dataUrl = `data:image/svg+xml;base64,${base64}`;
          setImageSrc(dataUrl);
        }
        root.unmount();
        document.body.removeChild(svgContainer);
      }, 100);
    } else if (src && typeof src === 'object' && 'default' in src) {
      setImageSrc(src.default);
    }
  }, [src]);

  const commonStyle = { cursor: 'pointer', maxWidth: '100%', height: 'auto' };

  return (
    <>
      {typeof src === 'function' ? (
        <div
          onClick={() => setOpen(true)}
          style={{ ...commonStyle, display: 'block' }}
          className={className}
          title={title}
        >
          {React.createElement(
            src as React.ComponentType<React.SVGProps<SVGSVGElement>>,
            { role: 'img', 'aria-label': alt, style: commonStyle },
          )}
        </div>
      ) : (
        <img
          src={imageSrc}
          alt={alt}
          title={title}
          className={className}
          onClick={() => setOpen(true)}
          style={commonStyle}
        />
      )}
      <Lightbox
        open={open}
        close={() => setOpen(false)}
        slides={[{ src: imageSrc, alt }]}
        plugins={[Zoom]}
        zoom={{
          scrollToZoom: true,
          maxZoomPixelRatio: 2,
          doubleTapDelay: 0,
        }}
        styles={{
          container: {
            '--yarl__color_backdrop': backdropColor,
          },
        }}
      />
    </>
  );
};

export default LightboxImage;
