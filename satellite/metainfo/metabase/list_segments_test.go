// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"bytes"
	"sort"
	"testing"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestListSegments(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts:     metabase.ListSegments{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Invalid limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List no segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListSegmentsResult{},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expectedObject := createObject(ctx, t, db, obj, 10)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			expectedRawSegments := make([]metabase.RawSegment, 10)
			expectedSegments := make([]metabase.Segment, 10)
			for i := range expectedSegments {
				expectedSegment.Position.Index = uint32(i)
				expectedSegments[i] = expectedSegment
				expectedRawSegments[i] = metabase.RawSegment(expectedSegment)
			}

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    10,
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments[:1],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 1,
					},
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments[2:4],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 10,
					},
				},
				Result: metabase.ListSegmentsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Part:  1,
						Index: 10,
					},
				},
				Result: metabase.ListSegmentsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("List segments from unordered parts", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			var testCases = []struct {
				segments []metabase.SegmentPosition
			}{
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 0, Index: 0},
					{Part: 1, Index: 0},
					{Part: 2, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 0, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 0, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 3, Index: 0},
				}},
			}

			expectedSegment := metabase.Segment{
				StreamID:          obj.StreamID,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			for _, tc := range testCases {
				obj := randObjectStream()

				BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   defaultTestEncryption,
					},
					Version: obj.Version,
				}.Check(ctx, t, db)

				for i, segmentPosition := range tc.segments {
					BeginSegment{
						Opts: metabase.BeginSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{byte(i + 1)},
							Pieces: []metabase.Piece{{
								Number:      1,
								StorageNode: testrand.NodeID(),
							}},
						},
					}.Check(ctx, t, db)

					CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{1},
							Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

							EncryptedKey:      []byte{3},
							EncryptedKeyNonce: []byte{4},

							EncryptedSize: 1024,
							PlainSize:     512,
							PlainOffset:   0,
							Redundancy:    defaultTestRedundancy,
						},
					}.Check(ctx, t, db)
				}

				CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				expectedSegments := make([]metabase.Segment, 4)
				for i := range expectedSegments {
					expectedSegments[i] = expectedSegment
					expectedSegments[i].StreamID = obj.StreamID
					expectedSegments[i].Position.Part = uint32(i)
				}

				ListSegments{
					Opts: metabase.ListSegments{
						StreamID: obj.StreamID,
						Limit:    0,
					},
					Result: metabase.ListSegmentsResult{
						Segments: expectedSegments,
					},
				}.Check(ctx, t, db)
			}
		})
	})
}

func TestListObjectsSegments(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("StreamIDs list is empty", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListObjectsSegments{
				Opts:     metabase.ListObjectsSegments{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamIDs list is empty",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("StreamIDs list contains empty ID", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListObjectsSegments{
				Opts: metabase.ListObjectsSegments{
					StreamIDs: []uuid.UUID{{}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing: index 0",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List objects segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expectedObject01 := createObject(ctx, t, db, randObjectStream(), 1)
			expectedObject02 := createObject(ctx, t, db, randObjectStream(), 5)
			expectedObject03 := createObject(ctx, t, db, randObjectStream(), 3)

			expectedSegments := []metabase.Segment{}
			expectedRawSegments := []metabase.RawSegment{}

			objects := []metabase.Object{expectedObject01, expectedObject02, expectedObject03}

			sort.Slice(objects, func(i, j int) bool {
				return bytes.Compare(objects[i].StreamID[:], objects[j].StreamID[:]) < 0
			})

			addSegments := func(object metabase.Object) {
				for i := 0; i < int(object.SegmentCount); i++ {
					segment := metabase.Segment{
						StreamID: object.StreamID,
						Position: metabase.SegmentPosition{
							Index: uint32(i),
						},
						RootPieceID:       storj.PieceID{1},
						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedSize:     1024,
						PlainSize:         512,
						Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
						Redundancy:        defaultTestRedundancy,
					}
					expectedSegments = append(expectedSegments, segment)
					expectedRawSegments = append(expectedRawSegments, metabase.RawSegment(segment))
				}
			}

			for _, object := range objects {
				addSegments(object)
			}

			ListObjectsSegments{
				Opts: metabase.ListObjectsSegments{
					StreamIDs: []uuid.UUID{
						expectedObject01.StreamID,
						expectedObject02.StreamID,
						expectedObject03.StreamID,
					},
				},
				Result: metabase.ListObjectsSegmentsResult{
					Segments: expectedSegments,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject01),
					metabase.RawObject(expectedObject02),
					metabase.RawObject(expectedObject03),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})
	})
}
