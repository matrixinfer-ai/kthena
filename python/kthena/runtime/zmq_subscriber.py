# Copyright The Volcano Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import asyncio
import logging
from datetime import datetime
from typing import Optional, Any, Union, Tuple

import msgspec
import zmq
import zmq.asyncio
from msgspec.msgpack import Decoder

from kthena.runtime.events import (
    EventType, VLLMEventData, VLLMBlockStoredEvent,
    VLLMBlockRemovedEvent, VLLMAllBlocksClearedEvent, get_event_publisher
)
from kthena.runtime.vllm_config import get_vllm_config

logger = logging.getLogger(__name__)


class EventBatch(msgspec.Struct, array_like=True, omit_defaults=True, gc=False):
    ts: float
    events: list[Any]
    data_parallel_rank: Optional[int] = None


class KVCacheEvent(
    msgspec.Struct, array_like=True, omit_defaults=True, gc=False, tag=True
):
    pass


class BlockStored(KVCacheEvent):
    block_hashes: list[int]
    parent_block_hash: Optional[int]
    token_ids: list[int]
    block_size: int
    lora_id: Optional[int]


class BlockRemoved(KVCacheEvent):
    block_hashes: list[int]


class AllBlocksCleared(KVCacheEvent):
    pass


class KVEventBatch(EventBatch):
    events: list[Union[BlockStored, BlockRemoved, AllBlocksCleared]]


class VLLMZMQSubscriber:

    def __init__(self, pod_identifier: str, model_name: str):
        self.config = get_vllm_config(pod_identifier, model_name)
        self.context: Optional[zmq.asyncio.Context] = None
        self.socket: Optional[zmq.asyncio.Socket] = None
        self.running = False
        self.event_publisher = get_event_publisher()
        self.decoder = Decoder(type=KVEventBatch)
        self._connection_lock = asyncio.Lock()
        self._shutdown_event = asyncio.Event()

    async def start(self) -> None:
        async with self._connection_lock:
            if self.running:
                logger.warning("vLLM ZMQ subscriber is already running")
                return

            self.running = True
            self._shutdown_event.clear()

        retry_count = 0
        while self.running and (self.config.zmq_max_retries == -1 or retry_count < self.config.zmq_max_retries):
            try:
                await self._run_subscriber()
                break
            except asyncio.CancelledError:
                logger.info("vLLM ZMQ subscriber cancelled")
                break
            except Exception as e:
                retry_count += 1
                logger.error(f"vLLM ZMQ subscriber error (attempt {retry_count}): {e}")

                if self.running and (self.config.zmq_max_retries == -1 or retry_count < self.config.zmq_max_retries):
                    try:
                        await asyncio.wait_for(self._shutdown_event.wait(), timeout=self.config.zmq_retry_interval)
                        break
                    except asyncio.TimeoutError:
                        continue

        logger.info("vLLM ZMQ subscriber stopped")

    async def stop(self) -> None:
        async with self._connection_lock:
            if not self.running:
                return

            self.running = False
            self._shutdown_event.set()

            await self._cleanup_connection()

    async def _cleanup_connection(self) -> None:
        if self.socket:
            try:
                self.socket.close()
            except (OSError, RuntimeError) as e:
                logger.warning(f"Error closing ZMQ socket: {e}")
            finally:
                self.socket = None

        if self.context:
            try:
                self.context.term()
            except (OSError, RuntimeError) as e:
                logger.warning(f"Error terminating ZMQ context: {e}")
            finally:
                self.context = None

    async def _run_subscriber(self) -> None:
        try:
            self.context = zmq.asyncio.Context()
            self.socket = self.context.socket(zmq.SUB)

            self.socket.connect(self.config.zmq_endpoint)
            self.socket.setsockopt_string(zmq.SUBSCRIBE, self.config.zmq_topic_filter)
            self.socket.setsockopt(zmq.RCVTIMEO, self.config.zmq_poll_timeout)

            logger.info(f"Connected to ZMQ endpoint: {self.config.zmq_endpoint}")

            while self.running:
                try:
                    parts = await self.socket.recv_multipart(zmq.NOBLOCK)

                    if not self._validate_message_parts(parts):
                        continue

                    topic, payload = self._extract_message_data(parts)
                    if not topic or topic != "kv-events":
                        continue

                    await self._process_message(payload, self.config.pod_identifier, self.config.model_name)

                except zmq.Again:
                    await asyncio.sleep(0.001)
                    continue
                except asyncio.CancelledError:
                    logger.info("ZMQ subscriber cancelled")
                    break
                except Exception as e:
                    logger.error(f"Error receiving vLLM ZMQ message: {e}")
                    if not self.running:
                        break
                    await asyncio.sleep(0.1)

        except Exception as e:
            logger.error(f"Error in ZMQ subscriber: {e}")
            raise
        finally:
            await self._cleanup_connection()

    @staticmethod
    def _validate_message_parts(parts) -> bool:
        return len(parts) == 3

    @staticmethod
    def _extract_message_data(parts) -> Tuple[Optional[str], Optional[bytes]]:
        try:
            topic = parts[0].decode('utf-8')
            payload = parts[2]
            return topic, payload
        except (UnicodeDecodeError, IndexError, AttributeError) as e:
            logger.warning(f"Error extracting message data: {e}")
            return None, None

    async def _process_message(self, payload: bytes, pod_identifier: str, model_name: str) -> None:
        if not payload:
            logger.warning("Empty payload received")
            return

        try:
            event_batch = self.decoder.decode(payload)

            if not event_batch or not hasattr(event_batch, 'events'):
                logger.warning("Invalid event batch structure")
                return

            events_count = len(event_batch.events) if event_batch.events else 0
            logger.info(
                f"Received event batch: ts={event_batch.ts}, events_count={events_count}, dp_rank={event_batch.data_parallel_rank}")

            if events_count == 0:
                logger.info("Empty event batch received")
                return

            for i, event in enumerate(event_batch.events):
                try:
                    logger.info(f"Processing event {i}: type={type(event).__name__}")
                    await self._process_event(event, event_batch.ts, pod_identifier, model_name,
                                              event_batch.data_parallel_rank)
                except (ValueError, TypeError, AttributeError) as e:
                    logger.error(f"Error processing event {i}: {e}")
                except Exception as e:
                    logger.error(f"Unexpected error processing event {i}: {e}", exc_info=True)

        except Exception as e:
            logger.error(f"Error processing message: {e}", exc_info=True)

    async def _process_event(self, event: Any, timestamp: float,
                             pod_identifier: str, model_name: str,
                             data_parallel_rank: Optional[int]) -> None:
        if not event:
            logger.warning("Empty event received")
            return

        try:
            if isinstance(event, BlockStored):

                vllm_event = VLLMBlockStoredEvent(
                    block_hashes=event.block_hashes,
                    parent_block_hash=event.parent_block_hash,
                    token_ids=event.token_ids,
                    block_size=event.block_size,
                    lora_id=event.lora_id
                )
                event_type = EventType.VLLM_BLOCK_STORED

            elif isinstance(event, BlockRemoved):
                vllm_event = VLLMBlockRemovedEvent(
                    block_hashes=event.block_hashes
                )
                event_type = EventType.VLLM_BLOCK_REMOVED

            elif isinstance(event, AllBlocksCleared):
                vllm_event = VLLMAllBlocksClearedEvent()
                event_type = EventType.VLLM_ALL_BLOCKS_CLEARED

            else:
                logger.debug(f"Unknown vLLM event type: {type(event)}")
                return

            event_data_obj = VLLMEventData(
                event_type=event_type,
                timestamp=datetime.fromtimestamp(timestamp),
                model_name=model_name,
                pod_identifier=pod_identifier,
                data_parallel_rank=data_parallel_rank,
                vllm_event=vllm_event
            )

            await self.event_publisher.publish(event_data_obj)

        except Exception as e:
            logger.error(f"Error processing vLLM event: {e}", exc_info=True)


def get_vllm_zmq_subscriber(pod_identifier: str, model_name: str) -> VLLMZMQSubscriber:
    return VLLMZMQSubscriber(pod_identifier, model_name)
