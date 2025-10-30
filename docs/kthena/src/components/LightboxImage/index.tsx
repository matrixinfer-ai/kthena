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

  // Prevent layout shift when lightbox opens by compensating for scrollbar
  React.useEffect(() => {
    if (open) {
      const scrollbarWidth =
        window.innerWidth - document.documentElement.clientWidth;
      document.body.style.paddingRight = `${scrollbarWidth}px`;
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.paddingRight = '';
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.paddingRight = '';
      document.body.style.overflow = '';
    };
  }, [open]);

  // Handle different src types - FIXED VERSION
  React.useEffect(() => {
    const handleSrcConversion = async () => {
      if (typeof src === 'string') {
        setImageSrc(src);
      } else if (typeof src === 'function') {
        // For SVG components, use a simpler approach
        try {
          // Create a temporary container
          const tempDiv = document.createElement('div');
          const RootComponent = src;

          // Use ReactDOM to render the component temporarily
          const { createRoot } = await import('react-dom/client');
          const root = createRoot(tempDiv);

          root.render(React.createElement(RootComponent));

          // Wait for next tick to ensure rendering is complete
          setTimeout(() => {
            const svgElement = tempDiv.querySelector('svg');
            if (svgElement) {
              // Clone the SVG to avoid modifying the original
              const svgClone = svgElement.cloneNode(true) as SVGElement;

              // Ensure proper attributes
              svgClone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');

              const svgString = new XMLSerializer().serializeToString(svgClone);
              const dataUrl = `data:image/svg+xml;base64,${btoa(unescape(encodeURIComponent(svgString)))}`;
              setImageSrc(dataUrl);
            }

            // Cleanup
            root.unmount();
          }, 0);
        } catch (error) {
          console.error('Error converting SVG component to data URL:', error);
        }
      } else if (src && typeof src === 'object' && 'default' in src) {
        setImageSrc(src.default);
      }
    };

    handleSrcConversion();
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
          onError={(e) => {
            console.error('Error loading image:', imageSrc);
          }}
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
        carousel={{
          padding: '0px',
        }}
        controller={{
          closeOnBackdropClick: true,
        }}
      />
    </>
  );
};

export default LightboxImage;
