package taskservice

import (
 "context"

 "go.mongodb.org/mongo-driver/bson"
 "go.mongodb.org/mongo-driver/bson/primitive"
 "go.mongodb.org/mongo-driver/mongo"
 "go.mongodb.org/mongo-driver/mongo/options"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
 //"google.golang.org/protobuf/types/known/timestamppb"

 pb "github.com/danielteferadeti/grpc_practice/proto"
)

type TaskService struct {
 pb.UnimplementedTaskServiceServer
 collection *mongo.Collection
}

func NewTaskService(collection *mongo.Collection) *TaskService {
 return &TaskService{collection: collection}
}

func (s *TaskService) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
 task := &pb.Task{
  Title:       req.Title,
  Description: req.Description,
  Completed:   false,
  DueDate:     req.DueDate,
 }

 res, err := s.collection.InsertOne(ctx, task)
 if err != nil {
  return nil, status.Errorf(codes.Internal, "Failed to create task: %v", err)
 }

 oid, ok := res.InsertedID.(primitive.ObjectID)
 if !ok {
  return nil, status.Errorf(codes.Internal, "Failed to convert InsertedID to ObjectID")
 }

 task.Id = oid.Hex()
 return task, nil
}

func (s *TaskService) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.Task, error) {
 oid, err := primitive.ObjectIDFromHex(req.Id)
 if err != nil {
  return nil, status.Errorf(codes.InvalidArgument, "Invalid ID")
 }

 var task pb.Task
 err = s.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&task)
 if err != nil {
  if err == mongo.ErrNoDocuments {
   return nil, status.Errorf(codes.NotFound, "Task not found")
  }
  return nil, status.Errorf(codes.Internal, "Failed to get task: %v", err)
 }

 task.Id = oid.Hex()
 return &task, nil
}

func (s *TaskService) UpdateTask(ctx context.Context, req *pb.UpdateTaskRequest) (*pb.Task, error) {
 oid, err := primitive.ObjectIDFromHex(req.Id)
 if err != nil {
  return nil, status.Errorf(codes.InvalidArgument, "Invalid ID")
 }

 update := bson.M{
  "$set": bson.M{
   "title":       req.Title,
   "description": req.Description,
   "completed":   req.Completed,
   "due_date":    req.DueDate.AsTime(),
  },
 }

 var updatedTask pb.Task
 err = s.collection.FindOneAndUpdate(
  ctx,
  bson.M{"_id": oid},
  update,
  options.FindOneAndUpdate().SetReturnDocument(options.After),
 ).Decode(&updatedTask)

 if err != nil {
  if err == mongo.ErrNoDocuments {
   return nil, status.Errorf(codes.NotFound, "Task not found")
  }
  return nil, status.Errorf(codes.Internal, "Failed to update task: %v", err)
 }

 updatedTask.Id = oid.Hex()
 return &updatedTask, nil
}

func (s *TaskService) DeleteTask(ctx context.Context, req *pb.DeleteTaskRequest) (*pb.DeleteTaskResponse, error) {
 oid, err := primitive.ObjectIDFromHex(req.Id)
 if err != nil {
  return nil, status.Errorf(codes.InvalidArgument, "Invalid ID")
 }

 res, err := s.collection.DeleteOne(ctx, bson.M{"_id": oid})
 if err != nil {
  return nil, status.Errorf(codes.Internal, "Failed to delete task: %v", err)
 }

 if res.DeletedCount == 0 {
  return nil, status.Errorf(codes.NotFound, "Task not found")
 }

 return &pb.DeleteTaskResponse{Success: true}, nil
}

func (s *TaskService) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
 var tasks []*pb.Task

 opts := options.Find().
  SetSkip(int64((req.Page - 1) * req.PageSize)).
  SetLimit(int64(req.PageSize))

 cursor, err := s.collection.Find(ctx, bson.M{}, opts)
 if err != nil {
  return nil, status.Errorf(codes.Internal, "Failed to list tasks: %v", err)
 }
 defer cursor.Close(ctx)

 for cursor.Next(ctx) {
  var task pb.Task
  if err := cursor.Decode(&task); err != nil {
   return nil, status.Errorf(codes.Internal, "Failed to decode task: %v", err)
  }
  task.Id = task.Id // Convert ObjectID to string
  tasks = append(tasks, &task)
 }

 if err := cursor.Err(); err != nil {
  return nil, status.Errorf(codes.Internal, "Cursor error: %v", err)
 }

 totalCount, err := s.collection.CountDocuments(ctx, bson.M{})
 if err != nil {
  return nil, status.Errorf(codes.Internal, "Failed to count tasks: %v", err)
 }

 return &pb.ListTasksResponse{
  Tasks:      tasks,
  TotalCount: int32(totalCount),
 }, nil
}