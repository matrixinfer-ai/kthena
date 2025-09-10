import React, { useState } from 'react';
import Lightbox from 'yet-another-react-lightbox';
import Zoom from 'yet-another-react-lightbox/plugins/zoom';
import 'yet-another-react-lightbox/styles.css';
import useBaseUrl from '@docusaurus/useBaseUrl';

interface LightboxImageProps {
  src: string;
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
  const imageSrc = useBaseUrl(src);

  return (
    <>
      <img
        src={imageSrc}
        alt={alt}
        title={title}
        className={className}
        onClick={() => setOpen(true)}
        style={{ cursor: 'pointer' }}
      />
      <Lightbox
        open={open}
        close={() => setOpen(false)}
        slides={[{ src: imageSrc, alt }]}
        plugins={[Zoom]}
        zoom={{
          scrollToZoom: true,
          maxZoomPixelRatio: 2, // Increased zoom level to 8x
          doubleTapDelay: 0,
        }}
        styles={{
          container: {
            // Example: semi-transparent black
            '--yarl__color_backdrop': 'rgba(0,0,0,0.8)',
          },
        }}
      />
    </>
  );
};

export default LightboxImage;
